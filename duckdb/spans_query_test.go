package duckdb

import (
	"context"
	"encoding/json"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// seedSpans inserts three spans across two services + two traces so the
// query handlers have something realistic to aggregate.
func seedSpans(t *testing.T, d *DuckDB) {
	t.Helper()
	trace1 := decodeHexTB(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	trace2 := decodeHexTB(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	mk := func(traceID []byte, spanHex, parentHex, service, name string, start, end uint64) *tracepb.ResourceSpans {
		return &tracepb.ResourceSpans{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
				{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: service}}},
			}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Scope: &commonpb.InstrumentationScope{Name: "test"},
				Spans: []*tracepb.Span{{
					TraceId:           traceID,
					SpanId:            decodeHexTB(t, spanHex),
					ParentSpanId:      decodeHexTB(t, parentHex),
					Name:              name,
					Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
					StartTimeUnixNano: start,
					EndTimeUnixNano:   end,
				}},
			}},
		}
	}

	data := &tracepb.TracesData{
		ResourceSpans: []*tracepb.ResourceSpans{
			mk(trace1, "1111111111111111", "", "svc-a", "root", 1_700_000_000_000_000_000, 1_700_000_000_300_000_000),
			mk(trace1, "2222222222222222", "1111111111111111", "svc-a", "child", 1_700_000_000_050_000_000, 1_700_000_000_200_000_000),
			mk(trace2, "3333333333333333", "", "svc-b", "solo", 1_700_000_001_000_000_000, 1_700_000_001_100_000_000),
		},
	}
	if err := d.InsertTracesData(context.Background(), data); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func decodeHexTB(t *testing.T, s string) []byte {
	t.Helper()
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi := unhexB(s[2*i])
		lo := unhexB(s[2*i+1])
		out[i] = hi<<4 | lo
	}
	return out
}

func unhexB(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

func TestTraceByID_ReturnsOrderedSpans(t *testing.T) {
	d := mustOpen(t)
	seedSpans(t, d)
	srv := NewServer(nil, d).Handler()

	rows, code := get(t, srv, "/api/traces/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	// Ordered by start_time ascending — root first, then child.
	if rows[0]["name"] != "root" {
		t.Errorf("rows[0].name = %v", rows[0]["name"])
	}
	if rows[1]["name"] != "child" {
		t.Errorf("rows[1].name = %v", rows[1]["name"])
	}
}

func TestTraceByID_IncludesResourceAttributes(t *testing.T) {
	d := mustOpen(t)
	seedSpans(t, d)
	srv := NewServer(nil, d).Handler()

	rows, code := get(t, srv, "/api/traces/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if len(rows) == 0 {
		t.Fatal("no rows")
	}
	raw, ok := rows[0]["resource_attributes"]
	if !ok {
		t.Fatalf("resource_attributes missing from response; keys=%v", keysOf(rows[0]))
	}
	if raw == nil {
		t.Fatal("resource_attributes is null; expected JSON object string")
	}
	s, ok := raw.(string)
	if !ok {
		t.Fatalf("resource_attributes = %T %v, want string", raw, raw)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("resource_attributes not valid JSON: %v (%s)", err, s)
	}
	if m["service.name"] != "svc-a" {
		t.Errorf("resource_attributes[service.name] = %v, want svc-a", m["service.name"])
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestTraceByID_UnknownTraceEmpty(t *testing.T) {
	d := mustOpen(t)
	seedSpans(t, d)
	srv := NewServer(nil, d).Handler()
	rows, code := get(t, srv, "/api/traces/00000000000000000000000000000099")
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0", len(rows))
	}
}

func TestServices_ReturnsDistinctServicesWithCounts(t *testing.T) {
	d := mustOpen(t)
	seedSpans(t, d)
	srv := NewServer(nil, d).Handler()

	rows, code := get(t, srv, "/api/services")
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}

	got := map[string]float64{}
	for _, r := range rows {
		got[r["service_name"].(string)] = r["span_count"].(float64)
	}
	if got["svc-a"] != 2 {
		t.Errorf("svc-a count = %v, want 2", got["svc-a"])
	}
	if got["svc-b"] != 1 {
		t.Errorf("svc-b count = %v, want 1", got["svc-b"])
	}
}

func TestSpansRecent_ReturnsMostRecentFirst(t *testing.T) {
	d := mustOpen(t)
	seedSpans(t, d)
	srv := NewServer(nil, d).Handler()

	rows, code := get(t, srv, "/api/spans/recent?limit=10")
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	// Ordered by start_time DESC: svc-b solo first (starts 1s later), then
	// svc-a child (50ms later than root), then svc-a root.
	if rows[0]["name"] != "solo" {
		t.Errorf("rows[0].name = %v", rows[0]["name"])
	}
	if rows[2]["name"] != "root" {
		t.Errorf("rows[2].name = %v", rows[2]["name"])
	}
}

func TestSpansRecent_FilterByService(t *testing.T) {
	d := mustOpen(t)
	seedSpans(t, d)
	srv := NewServer(nil, d).Handler()

	rows, code := get(t, srv, "/api/spans/recent?service=svc-b")
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0]["service_name"] != "svc-b" {
		t.Errorf("service_name = %v", rows[0]["service_name"])
	}
}

func TestSpansRecent_RespectsLimit(t *testing.T) {
	d := mustOpen(t)
	seedSpans(t, d)
	srv := NewServer(nil, d).Handler()

	rows, code := get(t, srv, "/api/spans/recent?limit=1")
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if len(rows) != 1 {
		t.Errorf("rows = %d, want 1", len(rows))
	}
}
