package duckdb

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestInsertTracesData_RoundtripSpan(t *testing.T) {
	d := mustOpen(t)
	ctx := context.Background()

	traceID := mustHex(t, "4bf92f3577b34da6a3ce929d0e0e4736")
	spanID := mustHex(t, "00f067aa0ba902b7")
	parentID := mustHex(t, "0011223344556677")

	data := &tracepb.TracesData{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						stringAttr("service.name", "worker-a"),
						intAttr("process.pid", 42),
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{
							Name:    "axon-hand",
							Version: "0.13.2",
						},
						Spans: []*tracepb.Span{
							{
								TraceId:           traceID,
								SpanId:            spanID,
								ParentSpanId:      parentID,
								Name:              "build.scaffold",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: 1_700_000_000_000_000_000,
								EndTimeUnixNano:   1_700_000_000_500_000_000,
								Attributes: []*commonpb.KeyValue{
									stringAttr("prd.name", "portfolio"),
									boolAttr("cached", true),
								},
								Status: &tracepb.Status{
									Code:    tracepb.Status_STATUS_CODE_OK,
									Message: "ok",
								},
							},
						},
					},
				},
			},
		},
	}

	if err := d.InsertTracesData(ctx, data); err != nil {
		t.Fatalf("insert: %v", err)
	}

	rows, err := d.DB().QueryContext(ctx, `
		SELECT trace_id, span_id, parent_span_id, name, kind, service_name,
		       scope_name, scope_version, duration_ns, status_code,
		       resource_attributes, attributes
		FROM spans`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("no rows")
	}
	var (
		traceHex, spanHex, parentHex, name, kind, svc string
		scopeName, scopeVersion, statusCode           string
		duration                                      uint64
		resourceAttrs, attrs                          string
	)
	if err := rows.Scan(&traceHex, &spanHex, &parentHex, &name, &kind, &svc,
		&scopeName, &scopeVersion, &duration, &statusCode,
		&resourceAttrs, &attrs); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if traceHex != hex.EncodeToString(traceID) {
		t.Errorf("trace_id = %q", traceHex)
	}
	if spanHex != hex.EncodeToString(spanID) {
		t.Errorf("span_id = %q", spanHex)
	}
	if parentHex != hex.EncodeToString(parentID) {
		t.Errorf("parent_span_id = %q", parentHex)
	}
	if name != "build.scaffold" {
		t.Errorf("name = %q", name)
	}
	if kind != "SPAN_KIND_INTERNAL" {
		t.Errorf("kind = %q", kind)
	}
	if svc != "worker-a" {
		t.Errorf("service_name = %q", svc)
	}
	if scopeName != "axon-hand" || scopeVersion != "0.13.2" {
		t.Errorf("scope = %q/%q", scopeName, scopeVersion)
	}
	if duration != 500_000_000 {
		t.Errorf("duration_ns = %d", duration)
	}
	if statusCode != "STATUS_CODE_OK" {
		t.Errorf("status_code = %q", statusCode)
	}

	var resourceMap map[string]any
	if err := json.Unmarshal([]byte(resourceAttrs), &resourceMap); err != nil {
		t.Fatalf("resource_attributes not valid JSON: %v (%s)", err, resourceAttrs)
	}
	if resourceMap["service.name"] != "worker-a" {
		t.Errorf("resource[service.name] = %v", resourceMap["service.name"])
	}
	if resourceMap["process.pid"].(float64) != 42 {
		t.Errorf("resource[process.pid] = %v", resourceMap["process.pid"])
	}

	var attrMap map[string]any
	if err := json.Unmarshal([]byte(attrs), &attrMap); err != nil {
		t.Fatalf("attributes not valid JSON: %v (%s)", err, attrs)
	}
	if attrMap["prd.name"] != "portfolio" {
		t.Errorf("attributes[prd.name] = %v", attrMap["prd.name"])
	}
	if attrMap["cached"] != true {
		t.Errorf("attributes[cached] = %v", attrMap["cached"])
	}

	if rows.Next() {
		t.Error("unexpected second row")
	}
}

func TestInsertTracesData_MultipleSpansAtomic(t *testing.T) {
	d := mustOpen(t)
	ctx := context.Background()

	mkSpan := func(name string, id string) *tracepb.Span {
		return &tracepb.Span{
			TraceId:           mustHex(t, "00000000000000000000000000000001"),
			SpanId:            mustHex(t, id),
			Name:              name,
			Kind:              tracepb.Span_SPAN_KIND_CLIENT,
			StartTimeUnixNano: 1_700_000_000_000_000_000,
			EndTimeUnixNano:   1_700_000_000_100_000_000,
		}
	}

	data := &tracepb.TracesData{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{stringAttr("service.name", "svc")},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{Name: "s"},
						Spans: []*tracepb.Span{
							mkSpan("a", "aaaaaaaaaaaaaaaa"),
							mkSpan("b", "bbbbbbbbbbbbbbbb"),
							mkSpan("c", "cccccccccccccccc"),
						},
					},
				},
			},
		},
	}

	if err := d.InsertTracesData(ctx, data); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var count int
	if err := d.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM spans`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Errorf("rows = %d, want 3", count)
	}
}

func TestInsertTracesData_Empty(t *testing.T) {
	d := mustOpen(t)
	ctx := context.Background()

	if err := d.InsertTracesData(ctx, nil); err != nil {
		t.Fatalf("nil: %v", err)
	}
	if err := d.InsertTracesData(ctx, &tracepb.TracesData{}); err != nil {
		t.Fatalf("empty: %v", err)
	}
}

func TestAnyValueAny_AllKinds(t *testing.T) {
	cases := []struct {
		name string
		in   *commonpb.AnyValue
		want any
	}{
		{"nil", nil, nil},
		{"string", &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "x"}}, "x"},
		{"bool", &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: true}}, true},
		{"int", &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 7}}, int64(7)},
		{"double", &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 1.5}}, 1.5},
		{"bytes", &commonpb.AnyValue{Value: &commonpb.AnyValue_BytesValue{BytesValue: []byte{0xde, 0xad}}}, "dead"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := anyValueAny(tc.in); got != tc.want {
				t.Errorf("anyValueAny() = %v, want %v", got, tc.want)
			}
		})
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode hex %q: %v", s, err)
	}
	return b
}

func stringAttr(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func intAttr(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
}

func boolAttr(k string, v bool) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: v}}}
}
