package duckdb

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// spansDDL is the OTLP-native flat shape for a span: one row per span,
// with resource and scope attributes repeated on each row. JSON-shaped
// columns are stored as VARCHAR so consumers can opt into DuckDB's JSON
// functions without requiring the extension at schema-init time.
const spansDDL = `CREATE TABLE IF NOT EXISTS spans (
	trace_id VARCHAR,
	span_id VARCHAR,
	parent_span_id VARCHAR,
	trace_state VARCHAR,
	name VARCHAR,
	kind VARCHAR,
	service_name VARCHAR,
	scope_name VARCHAR,
	scope_version VARCHAR,
	start_time TIMESTAMP,
	end_time TIMESTAMP,
	duration_ns UBIGINT,
	status_code VARCHAR,
	status_message VARCHAR,
	resource_attributes VARCHAR DEFAULT '{}',
	scope_attributes VARCHAR DEFAULT '{}',
	attributes VARCHAR DEFAULT '{}',
	events VARCHAR DEFAULT '[]',
	links VARCHAR DEFAULT '[]'
)`

// initSpans is called from InitSchema.
func (d *DuckDB) initSpans(ctx context.Context) error {
	if _, err := d.db.ExecContext(ctx, spansDDL); err != nil {
		return fmt.Errorf("init spans: %w", err)
	}
	return nil
}

// InsertTracesData flattens an OTLP TracesData payload and inserts one row
// per span. Resource and scope attributes are serialized as JSON objects
// and repeated on each span row; at axon-look volumes this denormalization
// is cheap and keeps spans queries single-table.
//
// The insert runs inside a transaction so the batch is atomic.
func (d *DuckDB) InsertTracesData(ctx context.Context, data *tracepb.TracesData) error {
	if data == nil || len(data.ResourceSpans) == 0 {
		return nil
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO spans (
		trace_id, span_id, parent_span_id, trace_state, name, kind,
		service_name, scope_name, scope_version,
		start_time, end_time, duration_ns,
		status_code, status_message,
		resource_attributes, scope_attributes, attributes,
		events, links
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, rs := range data.ResourceSpans {
		resourceAttrs, serviceName := flattenResource(rs.Resource)
		for _, ss := range rs.ScopeSpans {
			scopeAttrs := attrsToJSON(ss.Scope.GetAttributes())
			scopeName := ss.Scope.GetName()
			scopeVersion := ss.Scope.GetVersion()
			for _, span := range ss.Spans {
				if err := insertSpan(ctx, stmt, span, serviceName, scopeName, scopeVersion, resourceAttrs, scopeAttrs); err != nil {
					return err
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func insertSpan(ctx context.Context, stmt *sql.Stmt, s *tracepb.Span, serviceName, scopeName, scopeVersion, resourceAttrs, scopeAttrs string) error {
	startTime := time.Unix(0, int64(s.StartTimeUnixNano)).UTC()
	endTime := time.Unix(0, int64(s.EndTimeUnixNano)).UTC()
	var duration uint64
	if s.EndTimeUnixNano >= s.StartTimeUnixNano {
		duration = s.EndTimeUnixNano - s.StartTimeUnixNano
	}
	statusCode := ""
	statusMessage := ""
	if s.Status != nil {
		statusCode = s.Status.Code.String()
		statusMessage = s.Status.Message
	}

	_, err := stmt.ExecContext(ctx,
		hex.EncodeToString(s.TraceId),
		hex.EncodeToString(s.SpanId),
		hex.EncodeToString(s.ParentSpanId),
		s.TraceState,
		s.Name,
		s.Kind.String(),
		serviceName,
		scopeName,
		scopeVersion,
		startTime,
		endTime,
		duration,
		statusCode,
		statusMessage,
		resourceAttrs,
		scopeAttrs,
		attrsToJSON(s.Attributes),
		eventsToJSON(s.Events),
		linksToJSON(s.Links),
	)
	if err != nil {
		return fmt.Errorf("insert span %s: %w", hex.EncodeToString(s.SpanId), err)
	}
	return nil
}

// flattenResource serializes resource attributes to a JSON object and
// extracts the service.name value as a convenience column.
func flattenResource(r *resourcepb.Resource) (attrsJSON, serviceName string) {
	if r == nil {
		return "{}", ""
	}
	for _, kv := range r.Attributes {
		if kv.Key == "service.name" {
			serviceName = anyValueString(kv.Value)
			break
		}
	}
	return attrsToJSON(r.Attributes), serviceName
}

// attrsToJSON renders an OTLP KeyValue list as a flat JSON object. Each
// value goes through anyValueAny so JSON keeps its native type (string,
// number, bool, array, object).
func attrsToJSON(kvs []*commonpb.KeyValue) string {
	if len(kvs) == 0 {
		return "{}"
	}
	m := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		m[kv.Key] = anyValueAny(kv.Value)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func eventsToJSON(events []*tracepb.Span_Event) string {
	if len(events) == 0 {
		return "[]"
	}
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		out = append(out, map[string]any{
			"time_unix_nano": e.TimeUnixNano,
			"name":           e.Name,
			"attributes":     kvsToMap(e.Attributes),
		})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func linksToJSON(links []*tracepb.Span_Link) string {
	if len(links) == 0 {
		return "[]"
	}
	out := make([]map[string]any, 0, len(links))
	for _, l := range links {
		out = append(out, map[string]any{
			"trace_id":    hex.EncodeToString(l.TraceId),
			"span_id":     hex.EncodeToString(l.SpanId),
			"trace_state": l.TraceState,
			"attributes":  kvsToMap(l.Attributes),
		})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func kvsToMap(kvs []*commonpb.KeyValue) map[string]any {
	if len(kvs) == 0 {
		return map[string]any{}
	}
	m := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		m[kv.Key] = anyValueAny(kv.Value)
	}
	return m
}

// anyValueAny returns the OTLP AnyValue as a Go native value (string,
// int64, float64, bool, []any, map[string]any, or nil).
func anyValueAny(v *commonpb.AnyValue) any {
	if v == nil {
		return nil
	}
	switch x := v.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_BoolValue:
		return x.BoolValue
	case *commonpb.AnyValue_IntValue:
		return x.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return x.DoubleValue
	case *commonpb.AnyValue_ArrayValue:
		if x.ArrayValue == nil {
			return []any{}
		}
		out := make([]any, 0, len(x.ArrayValue.Values))
		for _, av := range x.ArrayValue.Values {
			out = append(out, anyValueAny(av))
		}
		return out
	case *commonpb.AnyValue_KvlistValue:
		if x.KvlistValue == nil {
			return map[string]any{}
		}
		return kvsToMap(x.KvlistValue.Values)
	case *commonpb.AnyValue_BytesValue:
		return hex.EncodeToString(x.BytesValue)
	default:
		return nil
	}
}

// anyValueString is a convenience for cases where we expect a string and
// are willing to stringify other types as a fallback.
func anyValueString(v *commonpb.AnyValue) string {
	switch x := anyValueAny(v).(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return string(b)
	}
}
