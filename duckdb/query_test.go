package duckdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	look "github.com/benaskins/axon-look"
)

// seed inserts sample rows covering the tables that InitSchema creates.
func seed(t *testing.T, d *DuckDB) {
	t.Helper()
	h := &ingestHandler{db: d}

	success := true
	base := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	events := []look.Event{
		{Type: "run_started", Timestamp: base, RunID: "r1", AgentSlug: "bot", UserID: "u1", Description: "smoke"},
		{Type: "message", Timestamp: base, ConversationID: "c1", AgentSlug: "bot", UserID: "u1", Role: "user", PromptTokens: 100, RunID: "r1"},
		{Type: "message", Timestamp: base.Add(time.Second), ConversationID: "c1", AgentSlug: "bot", UserID: "u1", Role: "assistant", PromptTokens: 200, CompletionTokens: 50, DurationMs: 1200, RunID: "r1"},
		{Type: "tool_invocation", Timestamp: base.Add(2 * time.Second), ConversationID: "c1", AgentSlug: "bot", UserID: "u1", ToolName: "search", Success: &success, DurationMs: 300, RunID: "r1"},
		{Type: "memory_extracted", Timestamp: base, AgentSlug: "bot", UserID: "u1", MemoryType: "episodic", Importance: 0.7, RunID: "r1"},
		{Type: "relationship_snapshot", Timestamp: base, AgentSlug: "bot", UserID: "u1", Trust: 0.5, Intimacy: 0.3, RunID: "r1"},
		{Type: "consolidation_completed", Timestamp: base, AgentSlug: "bot", UserID: "u1", PatternsFound: 2, MemoriesMerged: 1, RunID: "r1"},
		{Type: "run_completed", Timestamp: base.Add(10 * time.Second), RunID: "r1", AgentSlug: "bot", UserID: "u1", Description: "smoke"},
	}
	body, _ := json.Marshal(events)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(string(body))))
	if w.Code != http.StatusAccepted {
		t.Fatalf("seed ingest: %d %s", w.Code, w.Body.String())
	}
}

func get(t *testing.T, h http.Handler, path string) ([]map[string]any, int) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Body.Len() == 0 || w.Code != http.StatusOK {
		return nil, w.Code
	}
	var rows []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(w.Body.String()), "\n") {
		if line == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		rows = append(rows, row)
	}
	return rows, w.Code
}

func TestServer_Routes(t *testing.T) {
	d := mustOpen(t)
	seed(t, d)
	h := NewServer(nil, d).Handler()

	t.Run("health", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})

	t.Run("stats", func(t *testing.T) {
		rows, code := get(t, h, "/api/agents/bot/stats")
		if code != http.StatusOK {
			t.Fatalf("status = %d", code)
		}
		if len(rows) != 1 {
			t.Fatalf("rows = %d, want 1", len(rows))
		}
		if int(rows[0]["total_messages"].(float64)) != 2 {
			t.Errorf("total_messages = %v, want 2", rows[0]["total_messages"])
		}
	})

	t.Run("tools", func(t *testing.T) {
		rows, code := get(t, h, "/api/agents/bot/tools")
		if code != http.StatusOK {
			t.Fatalf("status = %d", code)
		}
		if len(rows) != 1 || rows[0]["tool_name"] != "search" {
			t.Errorf("unexpected tools rows: %v", rows)
		}
		if int(rows[0]["successes"].(float64)) != 1 {
			t.Errorf("successes = %v, want 1", rows[0]["successes"])
		}
	})

	t.Run("memories", func(t *testing.T) {
		rows, code := get(t, h, "/api/agents/bot/memories")
		if code != http.StatusOK {
			t.Fatalf("status = %d", code)
		}
		if len(rows) == 0 || rows[0]["memory_type"] != "episodic" {
			t.Errorf("unexpected memories rows: %v", rows)
		}
	})

	t.Run("runs", func(t *testing.T) {
		rows, code := get(t, h, "/api/runs")
		if code != http.StatusOK {
			t.Fatalf("status = %d", code)
		}
		if len(rows) != 1 || rows[0]["run_id"] != "r1" {
			t.Errorf("unexpected runs rows: %v", rows)
		}
	})

	t.Run("run summary", func(t *testing.T) {
		rows, code := get(t, h, "/api/runs/r1/summary")
		if code != http.StatusOK {
			t.Fatalf("status = %d", code)
		}
		if len(rows) != 1 {
			t.Fatalf("rows = %d, want 1", len(rows))
		}
		if int(rows[0]["messages"].(float64)) != 2 {
			t.Errorf("messages = %v, want 2", rows[0]["messages"])
		}
		if int(rows[0]["tool_invocations"].(float64)) != 1 {
			t.Errorf("tool_invocations = %v, want 1", rows[0]["tool_invocations"])
		}
		if int(rows[0]["conversations"].(float64)) != 1 {
			t.Errorf("conversations = %v, want 1", rows[0]["conversations"])
		}
	})

	t.Run("missing run_id", func(t *testing.T) {
		// Hit the handler directly so we can force an empty path value,
		// which Go's ServeMux otherwise rewrites away via redirect.
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/evals/", nil)
		req.SetPathValue("run_id", "")
		(&evalsDetailHandler{db: d}).ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}

func TestParsePeriod(t *testing.T) {
	cases := []struct {
		in        string
		defDays   int
		wantN     int
		wantUnit  string
	}{
		{"7d", 30, 7, "DAY"},
		{"24h", 30, 24, "HOUR"},
		{"15m", 30, 15, "MINUTE"},
		{"", 30, 30, "DAY"},
		{"bad", 30, 30, "DAY"},
		{"0d", 30, 30, "DAY"},
	}
	for _, tc := range cases {
		n, unit := parsePeriod(tc.in, tc.defDays)
		if n != tc.wantN || unit != tc.wantUnit {
			t.Errorf("parsePeriod(%q, %d) = (%d, %s), want (%d, %s)", tc.in, tc.defDays, n, unit, tc.wantN, tc.wantUnit)
		}
	}
}

// Direct Query round-trip to exercise conversation query.
func TestConversationsQuery(t *testing.T) {
	d := mustOpen(t)
	seed(t, d)
	body, err := d.Query(context.Background(),
		"SELECT m.conversation_id, COUNT(*) AS messages FROM events_message m WHERE m.agent_slug = {slug:String} GROUP BY m.conversation_id",
		map[string]string{"slug": "bot"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !strings.Contains(string(body), `"conversation_id":"c1"`) {
		t.Errorf("expected c1 in output: %s", body)
	}
}
