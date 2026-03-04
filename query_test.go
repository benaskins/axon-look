package anal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockQuerier struct {
	results map[string][]byte // query substring → response
}

func (m *mockQuerier) Query(ctx context.Context, query string) ([]byte, error) {
	for substr, result := range m.results {
		if len(substr) == 0 || contains(query, substr) {
			return result, nil
		}
	}
	return []byte("[]"), nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestStatsHandler(t *testing.T) {
	q := &mockQuerier{
		results: map[string][]byte{
			"": []byte(`{"total_conversations":12,"total_messages":248,"total_prompt_tokens":150000,"total_completion_tokens":85000,"avg_duration_ms":3200}` + "\n"),
		},
	}
	handler := &statsHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/helper/stats", nil)
	req.SetPathValue("slug", "helper")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["total_conversations"] != float64(12) {
		t.Errorf("expected total_conversations=12, got %v", resp["total_conversations"])
	}
}

func TestStatsHandler_MissingSlug(t *testing.T) {
	handler := &statsHandler{db: &mockQuerier{results: map[string][]byte{}}}
	req := httptest.NewRequest(http.MethodGet, "/api/agents//stats", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestMessagesHandler(t *testing.T) {
	q := &mockQuerier{
		results: map[string][]byte{
			"": []byte(`{"day":"2026-03-01","total":15,"user_messages":8,"assistant_messages":7,"prompt_tokens":12000,"completion_tokens":8000}` + "\n" +
				`{"day":"2026-03-02","total":22,"user_messages":11,"assistant_messages":11,"prompt_tokens":18000,"completion_tokens":12000}` + "\n"),
		},
	}
	handler := &messagesHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/helper/messages?period=7d", nil)
	req.SetPathValue("slug", "helper")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestToolsHandler(t *testing.T) {
	q := &mockQuerier{
		results: map[string][]byte{
			"": []byte(`{"tool_name":"web_search","invocations":42,"successes":40,"avg_duration_ms":850}` + "\n"),
		},
	}
	handler := &toolsHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/helper/tools?period=30d", nil)
	req.SetPathValue("slug", "helper")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRelationshipHandler(t *testing.T) {
	q := &mockQuerier{
		results: map[string][]byte{
			"": []byte(`{"timestamp":"2026-03-01 00:00:00.000","trust":0.5,"intimacy":0.3,"autonomy":0.5,"reciprocity":0.5,"playfulness":0.4,"conflict":0.1}` + "\n"),
		},
	}
	handler := &relationshipHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/helper/relationship?period=90d", nil)
	req.SetPathValue("slug", "helper")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMemoriesHandler(t *testing.T) {
	q := &mockQuerier{
		results: map[string][]byte{
			"": []byte(`{"memory_type":"episodic","count":15,"avg_importance":0.72}` + "\n"),
		},
	}
	handler := &memoriesHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/helper/memories?period=30d", nil)
	req.SetPathValue("slug", "helper")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRunSummaryHandler(t *testing.T) {
	q := &mockQuerier{
		results: map[string][]byte{
			"": []byte(`{"run_id":"run-20260304-153000","messages":6,"tool_invocations":2,"conversations":3,"memories":1,"relationship_snapshots":1,"consolidations":0}` + "\n"),
		},
	}
	handler := &runSummaryHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-20260304-153000/summary", nil)
	req.SetPathValue("run_id", "run-20260304-153000")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["run_id"] != "run-20260304-153000" {
		t.Errorf("expected run_id, got %v", resp["run_id"])
	}
}

func TestRunSummaryHandler_MissingRunID(t *testing.T) {
	handler := &runSummaryHandler{db: &mockQuerier{results: map[string][]byte{}}}
	req := httptest.NewRequest(http.MethodGet, "/api/runs//summary", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestConversationsHandler(t *testing.T) {
	q := &mockQuerier{
		results: map[string][]byte{
			"": []byte(`{"conversation_id":"conv-1","messages":24,"prompt_tokens":15000,"completion_tokens":9000,"avg_duration_ms":2800,"tools_used":3}` + "\n"),
		},
	}
	handler := &conversationsHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/helper/conversations", nil)
	req.SetPathValue("slug", "helper")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
