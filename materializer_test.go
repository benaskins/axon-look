package look

import (
	"context"
	"strings"
	"testing"
	"time"

	fact "github.com/benaskins/axon-fact"
)

func TestSchemaToCreateTable(t *testing.T) {
	s := fact.Schema{
		Name: "eval_bfcl",
		Fields: []fact.Field{
			{Name: "timestamp", Type: fact.DateTime64},
			{Name: "run_id", Type: fact.String},
			{Name: "model", Type: fact.LowCardinalityString},
			{Name: "pass", Type: fact.Bool},
			{Name: "duration_ms", Type: fact.UInt32},
			{Name: "score", Type: fact.Float32},
		},
		OrderBy: []string{"model", "timestamp"},
	}

	ddl := schemaToCreateTable(s)

	if !strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS eval_bfcl") {
		t.Errorf("missing table name in DDL:\n%s", ddl)
	}
	if !strings.Contains(ddl, "timestamp DateTime64(3)") {
		t.Errorf("missing DateTime64(3) column:\n%s", ddl)
	}
	if !strings.Contains(ddl, "model LowCardinality(String)") {
		t.Errorf("missing LowCardinality column:\n%s", ddl)
	}
	if !strings.Contains(ddl, "pass Bool") {
		t.Errorf("missing Bool column:\n%s", ddl)
	}
	if !strings.Contains(ddl, "duration_ms UInt32") {
		t.Errorf("missing UInt32 column:\n%s", ddl)
	}
	if !strings.Contains(ddl, "score Float32") {
		t.Errorf("missing Float32 column:\n%s", ddl)
	}
	if !strings.Contains(ddl, "ORDER BY (model, timestamp)") {
		t.Errorf("missing ORDER BY clause:\n%s", ddl)
	}
}

func TestSchemaToCreateTable_NoOrderBy(t *testing.T) {
	s := fact.Schema{
		Name:   "test_table",
		Fields: []fact.Field{{Name: "id", Type: fact.String}},
	}
	ddl := schemaToCreateTable(s)
	if !strings.Contains(ddl, "ORDER BY tuple()") {
		t.Errorf("expected ORDER BY tuple(), got:\n%s", ddl)
	}
}

func TestFactToInsert(t *testing.T) {
	s := fact.Schema{
		Name: "eval_bfcl",
		Fields: []fact.Field{
			{Name: "timestamp", Type: fact.DateTime64},
			{Name: "model", Type: fact.LowCardinalityString},
			{Name: "pass", Type: fact.Bool},
			{Name: "duration_ms", Type: fact.UInt32},
		},
		OrderBy: []string{"model", "timestamp"},
	}

	ts := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	f := fact.Fact{
		Schema: "eval_bfcl",
		Data: map[string]any{
			"timestamp":   ts,
			"model":       "qwen3.5-122B",
			"pass":        true,
			"duration_ms": uint32(5500),
		},
	}

	query, params := factToInsert(s, f)

	if !strings.Contains(query, "INSERT INTO eval_bfcl") {
		t.Errorf("missing table name:\n%s", query)
	}
	if !strings.Contains(query, "{model:String}") {
		t.Errorf("LowCardinality should use String in param placeholder:\n%s", query)
	}
	if !strings.Contains(query, "{timestamp:DateTime64(3)}") {
		t.Errorf("missing DateTime64 placeholder:\n%s", query)
	}

	if params["timestamp"] != "2026-03-30 12:00:00.000" {
		t.Errorf("timestamp = %q", params["timestamp"])
	}
	if params["model"] != "qwen3.5-122B" {
		t.Errorf("model = %q", params["model"])
	}
	if params["pass"] != "true" {
		t.Errorf("pass = %q", params["pass"])
	}
	if params["duration_ms"] != "5500" {
		t.Errorf("duration_ms = %q", params["duration_ms"])
	}
}

func TestFactToInsert_NilValues(t *testing.T) {
	s := fact.Schema{
		Name: "test",
		Fields: []fact.Field{
			{Name: "name", Type: fact.String},
			{Name: "count", Type: fact.UInt32},
			{Name: "flag", Type: fact.Bool},
			{Name: "data", Type: fact.JSON},
		},
	}

	f := fact.Fact{
		Schema: "test",
		Data:   map[string]any{}, // all nil
	}

	_, params := factToInsert(s, f)

	if params["name"] != "" {
		t.Errorf("name zero = %q", params["name"])
	}
	if params["count"] != "0" {
		t.Errorf("count zero = %q", params["count"])
	}
	if params["flag"] != "false" {
		t.Errorf("flag zero = %q", params["flag"])
	}
	if params["data"] != "[]" {
		t.Errorf("data zero = %q", params["data"])
	}
}

func TestCHMaterializer_EnsureSchema(t *testing.T) {
	mock := &mockInserter{}
	m := NewCHMaterializer(mock)

	s := fact.Schema{
		Name: "eval_bfcl",
		Fields: []fact.Field{
			{Name: "model", Type: fact.LowCardinalityString},
			{Name: "pass", Type: fact.Bool},
		},
		OrderBy: []string{"model"},
	}

	err := m.EnsureSchema(context.Background(), s)
	if err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if len(mock.queries) != 1 {
		t.Fatalf("expected 1 exec, got %d", len(mock.queries))
	}
	if !strings.Contains(mock.queries[0], "CREATE TABLE IF NOT EXISTS eval_bfcl") {
		t.Errorf("expected CREATE TABLE, got:\n%s", mock.queries[0])
	}
}

func TestCHMaterializer_Materialize(t *testing.T) {
	mock := &mockInserter{}
	m := NewCHMaterializer(mock)

	s := fact.Schema{
		Name: "eval_bfcl",
		Fields: []fact.Field{
			{Name: "model", Type: fact.LowCardinalityString},
			{Name: "pass", Type: fact.Bool},
		},
		OrderBy: []string{"model"},
	}

	// Must EnsureSchema first
	_ = m.EnsureSchema(context.Background(), s)
	mock.queries = nil // reset
	mock.params = nil

	err := m.Materialize(context.Background(), fact.Fact{
		Schema: "eval_bfcl",
		Data:   map[string]any{"model": "qwen3.5", "pass": true},
	})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if len(mock.queries) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(mock.queries))
	}
	if !strings.Contains(mock.queries[0], "INSERT INTO eval_bfcl") {
		t.Errorf("expected INSERT, got:\n%s", mock.queries[0])
	}
	if mock.params[0]["model"] != "qwen3.5" {
		t.Errorf("model param = %q", mock.params[0]["model"])
	}
	if mock.params[0]["pass"] != "true" {
		t.Errorf("pass param = %q", mock.params[0]["pass"])
	}
}

func TestCHMaterializer_UnknownSchema(t *testing.T) {
	mock := &mockInserter{}
	m := NewCHMaterializer(mock)

	err := m.Materialize(context.Background(), fact.Fact{
		Schema: "nonexistent",
		Data:   map[string]any{},
	})
	if err == nil {
		t.Fatal("expected error for unknown schema")
	}
	if !strings.Contains(err.Error(), "unknown schema") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCHMaterializer_InterfaceCompliance(t *testing.T) {
	var _ fact.Materializer = (*CHMaterializer)(nil)
}

// mockInserter records Exec calls.
type mockInserter struct {
	queries []string
	params  []map[string]string
}

func (m *mockInserter) Exec(_ context.Context, query string, params map[string]string) error {
	m.queries = append(m.queries, query)
	m.params = append(m.params, params)
	return nil
}
