package duckdb

import (
	"context"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// seedScaffoldSpans inserts four spans across two scaffolds plus one
// scaffoldless span, so the scaffold query handlers have something to
// group and filter.
func seedScaffoldSpans(t *testing.T, d *DuckDB) {
	t.Helper()
	traceA := decodeHexTB(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	traceB := decodeHexTB(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	traceC := decodeHexTB(t, "cccccccccccccccccccccccccccccccc")

	mk := func(traceID []byte, spanHex, service, scaffoldID, name string, start, end uint64) *tracepb.ResourceSpans {
		attrs := []*commonpb.KeyValue{
			{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: service}}},
		}
		if scaffoldID != "" {
			attrs = append(attrs, &commonpb.KeyValue{
				Key:   "scaffold.id",
				Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: scaffoldID}},
			})
		}
		return &tracepb.ResourceSpans{
			Resource: &resourcepb.Resource{Attributes: attrs},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Scope: &commonpb.InstrumentationScope{Name: "test"},
				Spans: []*tracepb.Span{{
					TraceId:           traceID,
					SpanId:            decodeHexTB(t, spanHex),
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
			mk(traceA, "1111111111111111", "plan-lead", "scf_a", "agent.plan-lead", 1_700_000_000_000_000_000, 1_700_000_000_300_000_000),
			mk(traceA, "2222222222222222", "plan-lead", "scf_a", "llm.call", 1_700_000_000_050_000_000, 1_700_000_000_200_000_000),
			mk(traceB, "3333333333333333", "code-hand", "scf_a", "agent.code-hand", 1_700_000_001_000_000_000, 1_700_000_001_100_000_000),
			mk(traceC, "4444444444444444", "scan-lead", "scf_b", "agent.scan-lead", 1_700_000_002_000_000_000, 1_700_000_002_050_000_000),
			mk(traceC, "5555555555555555", "ad-hoc", "", "no-scaffold", 1_700_000_003_000_000_000, 1_700_000_003_010_000_000),
		},
	}
	if err := d.InsertTracesData(context.Background(), data); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestScaffolds_GroupsByScaffoldIDWithCountsAndServices(t *testing.T) {
	d := mustOpen(t)
	seedScaffoldSpans(t, d)
	srv := NewServer(nil, d).Handler()

	rows, code := get(t, srv, "/api/scaffolds")
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (scaffoldless spans filtered out)", len(rows))
	}

	got := map[string]map[string]any{}
	for _, r := range rows {
		got[r["scaffold_id"].(string)] = r
	}
	if got["scf_a"]["span_count"].(float64) != 3 {
		t.Errorf("scf_a span_count = %v, want 3", got["scf_a"]["span_count"])
	}
	if got["scf_b"]["span_count"].(float64) != 1 {
		t.Errorf("scf_b span_count = %v, want 1", got["scf_b"]["span_count"])
	}

	services := got["scf_a"]["services"].(string)
	if services == "" {
		t.Errorf("scf_a services empty")
	}
}

func TestScaffolds_OrdersByLastSeenDescending(t *testing.T) {
	d := mustOpen(t)
	seedScaffoldSpans(t, d)
	srv := NewServer(nil, d).Handler()

	rows, code := get(t, srv, "/api/scaffolds")
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if rows[0]["scaffold_id"] != "scf_b" {
		t.Errorf("rows[0].scaffold_id = %v, want scf_b (most recent)", rows[0]["scaffold_id"])
	}
}

func TestScaffoldSpans_ReturnsSpansForScaffoldInStartOrder(t *testing.T) {
	d := mustOpen(t)
	seedScaffoldSpans(t, d)
	srv := NewServer(nil, d).Handler()

	rows, code := get(t, srv, "/api/scaffolds/scf_a/spans")
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[0]["name"] != "agent.plan-lead" {
		t.Errorf("rows[0].name = %v, want agent.plan-lead (earliest)", rows[0]["name"])
	}
	if rows[2]["name"] != "agent.code-hand" {
		t.Errorf("rows[2].name = %v, want agent.code-hand (latest)", rows[2]["name"])
	}
}

func TestScaffoldSpans_UnknownScaffoldEmpty(t *testing.T) {
	d := mustOpen(t)
	seedScaffoldSpans(t, d)
	srv := NewServer(nil, d).Handler()

	rows, code := get(t, srv, "/api/scaffolds/scf_nope/spans")
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0", len(rows))
	}
}
