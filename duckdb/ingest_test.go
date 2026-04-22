package duckdb

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	look "github.com/benaskins/axon-look"
)

type execCall struct {
	query  string
	params map[string]string
}

type mockDB struct {
	execCalls []execCall
}

func (m *mockDB) Exec(ctx context.Context, query string, params map[string]string) error {
	m.execCalls = append(m.execCalls, execCall{query: query, params: params})
	return nil
}

func TestIngestHandler_MessageAndToolInvocation(t *testing.T) {
	db := &mockDB{}
	h := &ingestHandler{db: db}

	success := true
	events := []look.Event{
		{
			Type:             "message",
			Timestamp:        time.Date(2026, 4, 22, 14, 0, 0, 0, time.UTC),
			ConversationID:   "conv-1",
			AgentSlug:        "helper",
			UserID:           "u1",
			Role:             "assistant",
			PromptTokens:     1200,
			CompletionTokens: 450,
			DurationMs:       3200,
		},
		{
			Type:           "tool_invocation",
			Timestamp:      time.Date(2026, 4, 22, 14, 0, 1, 0, time.UTC),
			ConversationID: "conv-1",
			AgentSlug:      "helper",
			UserID:         "u1",
			ToolName:       "web_search",
			Success:        &success,
			DurationMs:     850,
		},
	}
	body, _ := json.Marshal(events)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body)))

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", w.Code, w.Body.String())
	}
	if len(db.execCalls) != 2 {
		t.Fatalf("exec calls = %d, want 2", len(db.execCalls))
	}
	if !strings.Contains(db.execCalls[0].query, "events_message") {
		t.Errorf("first exec not events_message: %s", db.execCalls[0].query)
	}
	if db.execCalls[0].params["agent_slug"] != "helper" {
		t.Errorf("agent_slug = %q, want \"helper\"", db.execCalls[0].params["agent_slug"])
	}
	if !strings.Contains(db.execCalls[1].query, "events_tool_invocation") {
		t.Errorf("second exec not events_tool_invocation: %s", db.execCalls[1].query)
	}
}

func TestIngestHandler_AllEventTypes(t *testing.T) {
	db := &mockDB{}
	h := &ingestHandler{db: db}

	success := true
	events := []look.Event{
		{Type: "message", Timestamp: time.Now(), AgentSlug: "bot", Role: "user"},
		{Type: "tool_invocation", Timestamp: time.Now(), AgentSlug: "bot", ToolName: "search", Success: &success},
		{Type: "conversation_started", Timestamp: time.Now(), AgentSlug: "bot", EventName: "started"},
		{Type: "memory_extracted", Timestamp: time.Now(), AgentSlug: "bot", MemoryType: "episodic", Importance: 0.8},
		{Type: "relationship_snapshot", Timestamp: time.Now(), AgentSlug: "bot", Trust: 0.7},
		{Type: "consolidation_completed", Timestamp: time.Now(), AgentSlug: "bot", PatternsFound: 3},
		{Type: "run_started", Timestamp: time.Now(), AgentSlug: "bot", RunID: "r1"},
		{Type: "run_completed", Timestamp: time.Now(), AgentSlug: "bot", RunID: "r1"},
	}
	body, _ := json.Marshal(events)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body)))

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}
	if len(db.execCalls) != 8 {
		t.Fatalf("exec calls = %d, want 8", len(db.execCalls))
	}
}

func TestIngestHandler_UnknownTypeSkipped(t *testing.T) {
	db := &mockDB{}
	h := &ingestHandler{db: db}
	body, _ := json.Marshal([]look.Event{{Type: "unknown", Timestamp: time.Now()}})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body)))
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", w.Code)
	}
	if len(db.execCalls) != 0 {
		t.Errorf("exec calls = %d, want 0", len(db.execCalls))
	}
}

func TestIngestHandler_InvalidBody(t *testing.T) {
	db := &mockDB{}
	h := &ingestHandler{db: db}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader([]byte("not json"))))
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// Regression: placeholders are `{name:Type}`, never interpolated values.
func TestIngestHandler_SQLInjectionRegression(t *testing.T) {
	db := &mockDB{}
	h := &ingestHandler{db: db}
	malicious := "' OR 1=1; --"
	events := []look.Event{{
		Type:           "message",
		Timestamp:      time.Now(),
		AgentSlug:      malicious,
		UserID:         "u",
		ConversationID: "c",
		Role:           "user",
	}}
	body, _ := json.Marshal(events)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body)))

	if len(db.execCalls) != 1 {
		t.Fatalf("exec calls = %d, want 1", len(db.execCalls))
	}
	if strings.Contains(db.execCalls[0].query, malicious) {
		t.Errorf("malicious value appeared in query string: %s", db.execCalls[0].query)
	}
	if db.execCalls[0].params["agent_slug"] != malicious {
		t.Errorf("params should hold malicious value safely, got: %s", db.execCalls[0].params["agent_slug"])
	}
}

// End-to-end ingest against a real in-memory DuckDB.
func TestIngest_EndToEnd(t *testing.T) {
	d := mustOpen(t)
	h := &ingestHandler{db: d}

	events := []look.Event{
		{
			Type:             "message",
			Timestamp:        time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
			ConversationID:   "c1",
			AgentSlug:        "bot",
			UserID:           "u1",
			Role:             "assistant",
			PromptTokens:     100,
			CompletionTokens: 50,
			DurationMs:       1000,
			RunID:            "r1",
		},
	}
	body, _ := json.Marshal(events)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body)))
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d", w.Code)
	}

	out, err := d.Query(context.Background(),
		"SELECT agent_slug FROM events_message WHERE agent_slug = {slug:String}",
		map[string]string{"slug": "bot"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !strings.Contains(string(out), `"agent_slug":"bot"`) {
		t.Errorf("expected inserted row in query output: %s", out)
	}
}
