package duckdb

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func mustOpen(t *testing.T) *DuckDB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := d.InitSchema(context.Background()); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return d
}

func TestInitSchema_CreatesAllTables(t *testing.T) {
	d := mustOpen(t)
	rows, err := d.DB().QueryContext(context.Background(),
		"SELECT table_name FROM duckdb_tables() WHERE schema_name = 'main' ORDER BY table_name")
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, name)
	}
	want := []string{
		"events_consolidation",
		"events_conversation",
		"events_eval",
		"events_memory",
		"events_message",
		"events_relationship",
		"events_run",
		"events_tool_invocation",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("tables = %v, want %v", got, want)
	}
}

func TestRewrite_ReplacesPlaceholdersPositionally(t *testing.T) {
	q := "SELECT * FROM t WHERE slug = {slug:String} AND n = {n:UInt32}"
	out, args, err := rewrite(q, map[string]string{"slug": "x", "n": "42"})
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if strings.Count(out, "?") != 2 {
		t.Errorf("want two ? placeholders, got: %s", out)
	}
	if strings.Contains(out, "{") {
		t.Errorf("placeholder not removed: %s", out)
	}
	if len(args) != 2 {
		t.Fatalf("want 2 args, got %d", len(args))
	}
	if args[0] != "x" {
		t.Errorf("arg[0] = %v, want \"x\"", args[0])
	}
	if args[1].(int64) != 42 {
		t.Errorf("arg[1] = %v, want 42", args[1])
	}
}

func TestRewrite_RepeatedPlaceholder(t *testing.T) {
	q := "SELECT {a:String}, {a:String}, {a:String}"
	out, args, err := rewrite(q, map[string]string{"a": "hello"})
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if strings.Count(out, "?") != 3 {
		t.Errorf("want three ? placeholders, got: %s", out)
	}
	if len(args) != 3 {
		t.Fatalf("want 3 args, got %d", len(args))
	}
	for i, a := range args {
		if a != "hello" {
			t.Errorf("arg[%d] = %v, want \"hello\"", i, a)
		}
	}
}

func TestRewrite_MissingParam(t *testing.T) {
	_, _, err := rewrite("SELECT {missing:String}", map[string]string{})
	if err == nil {
		t.Error("expected error for missing param")
	}
}

func TestCoerce_Types(t *testing.T) {
	cases := []struct {
		raw, typ string
		want     any
	}{
		{"42", "UInt32", int64(42)},
		{"-7", "Int32", int64(-7)},
		{"0.5", "Float32", 0.5},
		{"true", "Bool", true},
		{"hello", "String", "hello"},
	}
	for _, tc := range cases {
		got, err := coerce(tc.raw, tc.typ)
		if err != nil {
			t.Errorf("coerce(%q, %q): %v", tc.raw, tc.typ, err)
			continue
		}
		if got != tc.want {
			t.Errorf("coerce(%q, %q) = %v, want %v", tc.raw, tc.typ, got, tc.want)
		}
	}
}

func TestExecAndQuery_Roundtrip(t *testing.T) {
	d := mustOpen(t)
	ctx := context.Background()
	ts := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC).Format("2006-01-02 15:04:05.000")

	err := d.Exec(ctx,
		"INSERT INTO events_message (timestamp, conversation_id, agent_slug, user_id, role, prompt_tokens, completion_tokens, duration_ms, run_id) "+
			"VALUES ({ts:DateTime64(3)}, {cid:String}, {slug:String}, {uid:String}, {role:String}, {pt:UInt32}, {ct:UInt32}, {dur:UInt32}, {rid:String})",
		map[string]string{
			"ts": ts, "cid": "c1", "slug": "bot", "uid": "u1", "role": "assistant",
			"pt": "100", "ct": "50", "dur": "1200", "rid": "r1",
		})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}

	body, err := d.Query(ctx,
		"SELECT agent_slug, prompt_tokens, completion_tokens FROM events_message WHERE agent_slug = {slug:String}",
		map[string]string{"slug": "bot"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 row, got %d: %s", len(lines), body)
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatalf("unmarshal row: %v", err)
	}
	if row["agent_slug"] != "bot" {
		t.Errorf("agent_slug = %v, want \"bot\"", row["agent_slug"])
	}
	// JSON numbers unmarshal as float64 into map[string]any.
	if row["prompt_tokens"].(float64) != 100 {
		t.Errorf("prompt_tokens = %v, want 100", row["prompt_tokens"])
	}
}
