package look

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/benaskins/axon"
)

// Querier executes SELECT queries against ClickHouse.
type Querier interface {
	Query(ctx context.Context, query string, params map[string]string) ([]byte, error)
}

var validPeriod = regexp.MustCompile(`^\d+[dhm]$`)

// parsePeriod converts a period string like "7d", "30d", "24h" into a ClickHouse interval.
// Returns the number and unit separately for use in INTERVAL expressions.
func parsePeriod(s string, defaultDays int) (int, string) {
	if s == "" || !validPeriod.MatchString(s) {
		return defaultDays, "DAY"
	}
	n, _ := strconv.Atoi(s[:len(s)-1])
	if n <= 0 {
		return defaultDays, "DAY"
	}
	switch s[len(s)-1] {
	case 'h':
		return n, "HOUR"
	case 'm':
		return n, "MINUTE"
	default:
		return n, "DAY"
	}
}

func periodFilter(param string, defaultDays int) string {
	n, unit := parsePeriod(param, defaultDays)
	return fmt.Sprintf("timestamp >= now() - INTERVAL %d %s", n, unit)
}

// writeQueryResult executes a query and writes the raw JSON response.
func writeQueryResult(w http.ResponseWriter, r *http.Request, db Querier, query string, params map[string]string) {
	body, err := db.Query(r.Context(), query, params)
	if err != nil {
		slog.Error("query failed", "error", err)
		axon.WriteError(w, http.StatusInternalServerError, "query failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func requireSlug(r *http.Request) (string, bool) {
	slug := r.PathValue("slug")
	return slug, slug != ""
}

// statsHandler serves GET /api/agents/{slug}/stats
type statsHandler struct {
	db Querier
}

func (h *statsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, ok := requireSlug(r)
	if !ok {
		axon.WriteError(w, http.StatusBadRequest, "slug is required")
		return
	}

	query := `
		SELECT
			uniqExact(conversation_id) as total_conversations,
			count() as total_messages,
			sum(prompt_tokens) as total_prompt_tokens,
			sum(completion_tokens) as total_completion_tokens,
			avg(duration_ms) as avg_duration_ms
		FROM events_message
		WHERE agent_slug = {slug:String}
		FORMAT JSONEachRow`
	params := map[string]string{"slug": slug}

	writeQueryResult(w, r, h.db, query, params)
}

// messagesHandler serves GET /api/agents/{slug}/messages?period=7d
type messagesHandler struct {
	db Querier
}

func (h *messagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, ok := requireSlug(r)
	if !ok {
		axon.WriteError(w, http.StatusBadRequest, "slug is required")
		return
	}

	period := periodFilter(r.URL.Query().Get("period"), 7)
	query := fmt.Sprintf(`
		SELECT
			toDate(timestamp) as day,
			count() as total,
			countIf(role = 'user') as user_messages,
			countIf(role = 'assistant') as assistant_messages,
			sum(prompt_tokens) as prompt_tokens,
			sum(completion_tokens) as completion_tokens
		FROM events_message
		WHERE agent_slug = {slug:String} AND %s
		GROUP BY day
		ORDER BY day
		FORMAT JSONEachRow`, period)
	params := map[string]string{"slug": slug}

	writeQueryResult(w, r, h.db, query, params)
}

// toolsHandler serves GET /api/agents/{slug}/tools?period=30d
type toolsHandler struct {
	db Querier
}

func (h *toolsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, ok := requireSlug(r)
	if !ok {
		axon.WriteError(w, http.StatusBadRequest, "slug is required")
		return
	}

	period := periodFilter(r.URL.Query().Get("period"), 30)
	query := fmt.Sprintf(`
		SELECT
			tool_name,
			count() as invocations,
			countIf(success = true) as successes,
			avg(duration_ms) as avg_duration_ms
		FROM events_tool_invocation
		WHERE agent_slug = {slug:String} AND %s
		GROUP BY tool_name
		ORDER BY invocations DESC
		FORMAT JSONEachRow`, period)
	params := map[string]string{"slug": slug}

	writeQueryResult(w, r, h.db, query, params)
}

// relationshipHandler serves GET /api/agents/{slug}/relationship?period=90d
type relationshipHandler struct {
	db Querier
}

func (h *relationshipHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, ok := requireSlug(r)
	if !ok {
		axon.WriteError(w, http.StatusBadRequest, "slug is required")
		return
	}

	period := periodFilter(r.URL.Query().Get("period"), 90)
	query := fmt.Sprintf(`
		SELECT
			timestamp,
			trust,
			intimacy,
			autonomy,
			reciprocity,
			playfulness,
			conflict
		FROM events_relationship
		WHERE agent_slug = {slug:String} AND %s
		ORDER BY timestamp
		FORMAT JSONEachRow`, period)
	params := map[string]string{"slug": slug}

	writeQueryResult(w, r, h.db, query, params)
}

// memoriesHandler serves GET /api/agents/{slug}/memories?period=30d
type memoriesHandler struct {
	db Querier
}

func (h *memoriesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, ok := requireSlug(r)
	if !ok {
		axon.WriteError(w, http.StatusBadRequest, "slug is required")
		return
	}

	period := periodFilter(r.URL.Query().Get("period"), 30)
	query := fmt.Sprintf(`
		SELECT
			memory_type,
			count() as count,
			avg(importance) as avg_importance
		FROM events_memory
		WHERE agent_slug = {slug:String} AND %s
		GROUP BY memory_type
		ORDER BY count DESC
		FORMAT JSONEachRow`, period)
	params := map[string]string{"slug": slug}

	writeQueryResult(w, r, h.db, query, params)
}

// conversationsHandler serves GET /api/agents/{slug}/conversations
type conversationsHandler struct {
	db Querier
}

func (h *conversationsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, ok := requireSlug(r)
	if !ok {
		axon.WriteError(w, http.StatusBadRequest, "slug is required")
		return
	}

	period := periodFilter(r.URL.Query().Get("period"), 90)
	query := fmt.Sprintf(`
		SELECT
			m.conversation_id as conversation_id,
			count() as messages,
			sum(m.prompt_tokens) as prompt_tokens,
			sum(m.completion_tokens) as completion_tokens,
			avg(m.duration_ms) as avg_duration_ms,
			(SELECT count() FROM events_tool_invocation t WHERE t.conversation_id = m.conversation_id) as tools_used
		FROM events_message m
		WHERE m.agent_slug = {slug:String} AND %s
		GROUP BY m.conversation_id
		ORDER BY max(m.timestamp) DESC
		FORMAT JSONEachRow`, period)
	params := map[string]string{"slug": slug}

	writeQueryResult(w, r, h.db, query, params)
}

// evalsListHandler serves GET /api/evals — lists eval runs with aggregate scores.
type evalsListHandler struct {
	db Querier
}

func (h *evalsListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT
			run_id,
			min(timestamp) as timestamp,
			count() as scenarios,
			sum(passed) as passed,
			sum(failed) as failed,
			sum(total) as total
		FROM events_eval
		GROUP BY run_id
		ORDER BY timestamp DESC
		FORMAT JSONEachRow`

	writeQueryResult(w, r, h.db, query, nil)
}

// evalsDetailHandler serves GET /api/evals/{run_id} — full scenario details for a run.
type evalsDetailHandler struct {
	db Querier
}

func (h *evalsDetailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("run_id")
	if runID == "" {
		axon.WriteError(w, http.StatusBadRequest, "run_id is required")
		return
	}

	query := `
		SELECT
			run_id,
			scenario,
			response,
			duration_ms,
			tools_used,
			passed,
			failed,
			total,
			criteria
		FROM events_eval
		WHERE run_id = {run_id:String}
		ORDER BY timestamp
		FORMAT JSONEachRow`
	params := map[string]string{"run_id": runID}

	writeQueryResult(w, r, h.db, query, params)
}

// runsHandler serves GET /api/runs — lists available runs
type runsHandler struct {
	db Querier
}

func (h *runsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT
			run_id,
			min(timestamp) as started_at,
			max(timestamp) as completed_at,
			argMin(description, timestamp) as description,
			argMin(agent_slug, timestamp) as agent_slug
		FROM events_run
		GROUP BY run_id
		ORDER BY started_at DESC
		FORMAT JSONEachRow`

	writeQueryResult(w, r, h.db, query, nil)
}

// runSummaryHandler serves GET /api/runs/{run_id}/summary
type runSummaryHandler struct {
	db Querier
}

func (h *runSummaryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("run_id")
	if runID == "" {
		axon.WriteError(w, http.StatusBadRequest, "run_id is required")
		return
	}

	query := `
		SELECT
			{run_id:String} as run_id,
			(SELECT count() FROM events_message WHERE run_id = {run_id:String}) as messages,
			(SELECT count() FROM events_tool_invocation WHERE run_id = {run_id:String}) as tool_invocations,
			(SELECT uniqExact(conversation_id) FROM events_message WHERE run_id = {run_id:String}) as conversations,
			(SELECT count() FROM events_memory WHERE run_id = {run_id:String}) as memories,
			(SELECT count() FROM events_relationship WHERE run_id = {run_id:String}) as relationship_snapshots,
			(SELECT count() FROM events_consolidation WHERE run_id = {run_id:String}) as consolidations
		FORMAT JSONEachRow`
	params := map[string]string{"run_id": runID}

	writeQueryResult(w, r, h.db, query, params)
}
