package anal

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type mockClickHouse struct {
	execCalls []string
}

func (m *mockClickHouse) Exec(ctx context.Context, query string) error {
	m.execCalls = append(m.execCalls, query)
	return nil
}

func TestIngestHandler_MessageEvent(t *testing.T) {
	ch := &mockClickHouse{}
	handler := &ingestHandler{db: ch}

	success := true
	events := []Event{
		{
			Type:             "message",
			Timestamp:        time.Date(2026, 3, 4, 14, 0, 0, 0, time.UTC),
			ConversationID:   "conv-1",
			AgentSlug:        "helper",
			UserID:           "user1",
			Role:             "assistant",
			PromptTokens:     1200,
			CompletionTokens: 450,
			DurationMs:       3200,
		},
		{
			Type:           "tool_invocation",
			Timestamp:      time.Date(2026, 3, 4, 14, 0, 1, 0, time.UTC),
			ConversationID: "conv-1",
			AgentSlug:      "helper",
			UserID:         "user1",
			ToolName:       "web_search",
			Success:        &success,
			DurationMs:     850,
		},
	}

	body, _ := json.Marshal(events)
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	if len(ch.execCalls) != 2 {
		t.Fatalf("expected 2 exec calls, got %d", len(ch.execCalls))
	}

	if !strings.Contains(ch.execCalls[0], "events_message") {
		t.Errorf("expected insert into events_message, got: %s", ch.execCalls[0])
	}
	if !strings.Contains(ch.execCalls[1], "events_tool_invocation") {
		t.Errorf("expected insert into events_tool_invocation, got: %s", ch.execCalls[1])
	}
}

func TestIngestHandler_AllEventTypes(t *testing.T) {
	ch := &mockClickHouse{}
	handler := &ingestHandler{db: ch}

	success := true
	events := []Event{
		{Type: "message", Timestamp: time.Now(), AgentSlug: "bot", Role: "user"},
		{Type: "tool_invocation", Timestamp: time.Now(), AgentSlug: "bot", ToolName: "search", Success: &success},
		{Type: "conversation_started", Timestamp: time.Now(), AgentSlug: "bot", EventName: "started"},
		{Type: "memory_extracted", Timestamp: time.Now(), AgentSlug: "bot", MemoryType: "episodic", Importance: 0.8},
		{Type: "relationship_snapshot", Timestamp: time.Now(), AgentSlug: "bot", Trust: 0.7},
		{Type: "consolidation_completed", Timestamp: time.Now(), AgentSlug: "bot", PatternsFound: 3},
	}

	body, _ := json.Marshal(events)
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}

	if len(ch.execCalls) != 6 {
		t.Fatalf("expected 6 exec calls, got %d", len(ch.execCalls))
	}

	expectedTables := []string{
		"events_message",
		"events_tool_invocation",
		"events_conversation",
		"events_memory",
		"events_relationship",
		"events_consolidation",
	}
	for i, table := range expectedTables {
		if !strings.Contains(ch.execCalls[i], table) {
			t.Errorf("call %d: expected %s, got: %s", i, table, ch.execCalls[i])
		}
	}
}

func TestIngestHandler_EmptyBatch(t *testing.T) {
	ch := &mockClickHouse{}
	handler := &ingestHandler{db: ch}

	body, _ := json.Marshal([]Event{})
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}
	if len(ch.execCalls) != 0 {
		t.Errorf("expected no exec calls for empty batch")
	}
}

func TestIngestHandler_InvalidBody(t *testing.T) {
	ch := &mockClickHouse{}
	handler := &ingestHandler{db: ch}

	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestIngestHandler_UnknownEventType(t *testing.T) {
	ch := &mockClickHouse{}
	handler := &ingestHandler{db: ch}

	events := []Event{{Type: "unknown_type", Timestamp: time.Now()}}
	body, _ := json.Marshal(events)
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should still return 202 — skip unknown types
	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}
	if len(ch.execCalls) != 0 {
		t.Errorf("expected no exec calls for unknown event type")
	}
}
