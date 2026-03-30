package look

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type queryCall struct {
	query  string
	params map[string]string
}

type mockQuerier struct {
	results map[string][]byte // query substring → response
	calls   []queryCall
}

func (m *mockQuerier) Query(ctx context.Context, query string, params map[string]string) ([]byte, error) {
	m.calls = append(m.calls, queryCall{query: query, params: params})
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

	// Verify parameterized query
	if len(q.calls) != 1 {
		t.Fatalf("expected 1 query call, got %d", len(q.calls))
	}
	if q.calls[0].params["slug"] != "helper" {
		t.Errorf("expected slug=helper in params, got: %v", q.calls[0].params)
	}
	if strings.Contains(q.calls[0].query, "'helper'") {
		t.Errorf("slug should not be interpolated into query: %s", q.calls[0].query)
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

	if len(q.calls) != 1 {
		t.Fatalf("expected 1 query call, got %d", len(q.calls))
	}
	if q.calls[0].params["slug"] != "helper" {
		t.Errorf("expected slug=helper in params, got: %v", q.calls[0].params)
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

	if q.calls[0].params["slug"] != "helper" {
		t.Errorf("expected slug=helper in params, got: %v", q.calls[0].params)
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

	if q.calls[0].params["slug"] != "helper" {
		t.Errorf("expected slug=helper in params, got: %v", q.calls[0].params)
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

	if q.calls[0].params["slug"] != "helper" {
		t.Errorf("expected slug=helper in params, got: %v", q.calls[0].params)
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

	if q.calls[0].params["run_id"] != "run-20260304-153000" {
		t.Errorf("expected run_id in params, got: %v", q.calls[0].params)
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

func TestEvalsListHandler(t *testing.T) {
	q := &mockQuerier{
		results: map[string][]byte{
			"": []byte(`{"run_id":"run-20260304-170000","plan":"smoke.yaml","timestamp":"2026-03-04 17:00:00.000","scenarios":3,"passed":4,"failed":6,"total":10}` + "\n"),
		},
	}
	handler := &evalsListHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/evals", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// evalsListHandler passes nil params (no user input in query)
	if q.calls[0].params != nil {
		t.Errorf("expected nil params for evals list, got: %v", q.calls[0].params)
	}
}

func TestEvalsDetailHandler(t *testing.T) {
	q := &mockQuerier{
		results: map[string][]byte{
			"": []byte(`{"run_id":"run-20260304-170000","scenario":"greeting","response":"Hello!","duration_ms":2847,"tools_used":"[]","passed":1,"failed":2,"total":3,"criteria":"[{\"criterion\":\"min_length\",\"pass\":true,\"score\":1,\"reason\":\"ok\"}]"}` + "\n"),
		},
	}
	handler := &evalsDetailHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/evals/run-20260304-170000", nil)
	req.SetPathValue("run_id", "run-20260304-170000")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if q.calls[0].params["run_id"] != "run-20260304-170000" {
		t.Errorf("expected run_id in params, got: %v", q.calls[0].params)
	}
}

func TestEvalsDetailHandler_MissingRunID(t *testing.T) {
	handler := &evalsDetailHandler{db: &mockQuerier{results: map[string][]byte{}}}
	req := httptest.NewRequest(http.MethodGet, "/api/evals/", nil)
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

	if q.calls[0].params["slug"] != "helper" {
		t.Errorf("expected slug=helper in params, got: %v", q.calls[0].params)
	}
}

func TestBfclRunsHandler(t *testing.T) {
	q := &mockQuerier{
		results: map[string][]byte{
			"": []byte(`{"run_id":"bfcl-abc","model":"qwen3.5","provider":"local","timestamp":"2026-03-30","total":15,"passed":14,"failed":1,"accuracy":93.3,"avg_duration_ms":5500,"parameters":"{}"}` + "\n"),
		},
	}
	handler := &bfclRunsHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/evals/bfcl", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(q.calls[0].query, "eval_bfcl") {
		t.Errorf("expected query against eval_bfcl, got: %s", q.calls[0].query)
	}
}

func TestBfclRunsHandler_ModelFilter(t *testing.T) {
	q := &mockQuerier{results: map[string][]byte{"": []byte("{}\n")}}
	handler := &bfclRunsHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/evals/bfcl?model=qwen3.5", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if q.calls[0].params["model"] != "qwen3.5" {
		t.Errorf("expected model param, got: %v", q.calls[0].params)
	}
	if !strings.Contains(q.calls[0].query, "{model:String}") {
		t.Errorf("expected parameterized model filter, got: %s", q.calls[0].query)
	}
}

func TestBfclDetailHandler(t *testing.T) {
	q := &mockQuerier{results: map[string][]byte{"": []byte("{}\n")}}
	handler := &bfclDetailHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/evals/bfcl/bfcl-abc", nil)
	req.SetPathValue("run_id", "bfcl-abc")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if q.calls[0].params["run_id"] != "bfcl-abc" {
		t.Errorf("expected run_id param, got: %v", q.calls[0].params)
	}
}

func TestBfclDetailHandler_MissingRunID(t *testing.T) {
	handler := &bfclDetailHandler{db: &mockQuerier{results: map[string][]byte{}}}
	req := httptest.NewRequest(http.MethodGet, "/api/evals/bfcl/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBfclCompareHandler(t *testing.T) {
	q := &mockQuerier{results: map[string][]byte{"": []byte("{}\n")}}
	handler := &bfclCompareHandler{db: q}

	req := httptest.NewRequest(http.MethodGet, "/api/evals/bfcl/compare", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(q.calls[0].query, "eval_bfcl") {
		t.Errorf("expected query against eval_bfcl, got: %s", q.calls[0].query)
	}
}

func TestPeriodFilter_RejectsMaliciousInput(t *testing.T) {
	// Malicious period values should fall through to defaults
	result := periodFilter("1d; DROP TABLE", 7)
	expected := "timestamp >= now() - INTERVAL 7 DAY"
	if result != expected {
		t.Errorf("expected default filter %q for malicious input, got: %q", expected, result)
	}
}

func TestQueryHandler_SQLInjectionRegression(t *testing.T) {
	q := &mockQuerier{
		results: map[string][]byte{
			"": []byte(`{}` + "\n"),
		},
	}
	handler := &statsHandler{db: q}

	maliciousSlug := "' OR 1=1; --"
	req := httptest.NewRequest(http.MethodGet, "/api/agents/test/stats", nil)
	req.SetPathValue("slug", maliciousSlug)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(q.calls) != 1 {
		t.Fatalf("expected 1 query call, got %d", len(q.calls))
	}

	call := q.calls[0]

	// The query must use placeholders, NOT interpolated values
	if strings.Contains(call.query, maliciousSlug) {
		t.Errorf("SQL injection: malicious input found in query string: %s", call.query)
	}
	if !strings.Contains(call.query, "{slug:String}") {
		t.Errorf("expected parameterized placeholder in query, got: %s", call.query)
	}

	// The malicious value should be safely in the params map
	if call.params["slug"] != maliciousSlug {
		t.Errorf("expected malicious value in params map, got: %s", call.params["slug"])
	}
}
