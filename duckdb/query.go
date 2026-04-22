package duckdb

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/benaskins/axon"
)

// Querier executes SELECT queries against the backing DuckDB.
type Querier interface {
	Query(ctx context.Context, query string, params map[string]string) ([]byte, error)
}

var validPeriod = regexp.MustCompile(`^\d+[dhm]$`)

// parsePeriod converts a period string like "7d", "30d", "24h" into a
// count + DuckDB interval unit (DAY / HOUR / MINUTE).
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
	return fmt.Sprintf("timestamp >= NOW() - INTERVAL %d %s", n, unit)
}

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

type statsHandler struct{ db Querier }

func (h *statsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, ok := requireSlug(r)
	if !ok {
		axon.WriteError(w, http.StatusBadRequest, "slug is required")
		return
	}
	query := `
		SELECT
			COUNT(DISTINCT conversation_id) AS total_conversations,
			COUNT(*) AS total_messages,
			SUM(prompt_tokens) AS total_prompt_tokens,
			SUM(completion_tokens) AS total_completion_tokens,
			AVG(duration_ms) AS avg_duration_ms
		FROM events_message
		WHERE agent_slug = {slug:String}`
	writeQueryResult(w, r, h.db, query, map[string]string{"slug": slug})
}

type messagesHandler struct{ db Querier }

func (h *messagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, ok := requireSlug(r)
	if !ok {
		axon.WriteError(w, http.StatusBadRequest, "slug is required")
		return
	}
	period := periodFilter(r.URL.Query().Get("period"), 7)
	query := fmt.Sprintf(`
		SELECT
			CAST(timestamp AS DATE) AS day,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE role = 'user') AS user_messages,
			COUNT(*) FILTER (WHERE role = 'assistant') AS assistant_messages,
			SUM(prompt_tokens) AS prompt_tokens,
			SUM(completion_tokens) AS completion_tokens
		FROM events_message
		WHERE agent_slug = {slug:String} AND %s
		GROUP BY day
		ORDER BY day`, period)
	writeQueryResult(w, r, h.db, query, map[string]string{"slug": slug})
}

type toolsHandler struct{ db Querier }

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
			COUNT(*) AS invocations,
			COUNT(*) FILTER (WHERE success = true) AS successes,
			AVG(duration_ms) AS avg_duration_ms
		FROM events_tool_invocation
		WHERE agent_slug = {slug:String} AND %s
		GROUP BY tool_name
		ORDER BY invocations DESC`, period)
	writeQueryResult(w, r, h.db, query, map[string]string{"slug": slug})
}

type relationshipHandler struct{ db Querier }

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
		ORDER BY timestamp`, period)
	writeQueryResult(w, r, h.db, query, map[string]string{"slug": slug})
}

type memoriesHandler struct{ db Querier }

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
			COUNT(*) AS count,
			AVG(importance) AS avg_importance
		FROM events_memory
		WHERE agent_slug = {slug:String} AND %s
		GROUP BY memory_type
		ORDER BY count DESC`, period)
	writeQueryResult(w, r, h.db, query, map[string]string{"slug": slug})
}

type conversationsHandler struct{ db Querier }

func (h *conversationsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, ok := requireSlug(r)
	if !ok {
		axon.WriteError(w, http.StatusBadRequest, "slug is required")
		return
	}
	period := periodFilter(r.URL.Query().Get("period"), 90)
	query := fmt.Sprintf(`
		SELECT
			m.conversation_id AS conversation_id,
			COUNT(*) AS messages,
			SUM(m.prompt_tokens) AS prompt_tokens,
			SUM(m.completion_tokens) AS completion_tokens,
			AVG(m.duration_ms) AS avg_duration_ms,
			(SELECT COUNT(*) FROM events_tool_invocation t WHERE t.conversation_id = m.conversation_id) AS tools_used
		FROM events_message m
		WHERE m.agent_slug = {slug:String} AND %s
		GROUP BY m.conversation_id
		ORDER BY MAX(m.timestamp) DESC`, period)
	writeQueryResult(w, r, h.db, query, map[string]string{"slug": slug})
}

type evalsListHandler struct{ db Querier }

func (h *evalsListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT
			run_id,
			MIN(timestamp) AS timestamp,
			COUNT(*) AS scenarios,
			SUM(passed) AS passed,
			SUM(failed) AS failed,
			SUM(total) AS total
		FROM events_eval
		GROUP BY run_id
		ORDER BY timestamp DESC`
	writeQueryResult(w, r, h.db, query, nil)
}

type evalsDetailHandler struct{ db Querier }

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
		ORDER BY timestamp`
	writeQueryResult(w, r, h.db, query, map[string]string{"run_id": runID})
}

// bfclRunsHandler serves GET /api/evals/bfcl. The eval_bfcl table is
// managed outside InitSchema (same as in the ClickHouse package) and is
// expected to exist when this endpoint is queried.
type bfclRunsHandler struct{ db Querier }

func (h *bfclRunsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	modelFilter := ""
	params := map[string]string{}
	if m := r.URL.Query().Get("model"); m != "" {
		modelFilter = "WHERE model = {model:String}"
		params["model"] = m
	}
	query := fmt.Sprintf(`
		SELECT
			run_id,
			model,
			provider,
			MIN(timestamp) AS timestamp,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE pass = true) AS passed,
			COUNT(*) FILTER (WHERE pass = false) AS failed,
			ROUND(COUNT(*) FILTER (WHERE pass = true) * 100.0 / COUNT(*), 1) AS accuracy,
			AVG(duration_ms) AS avg_duration_ms,
			ANY_VALUE(parameters) AS parameters
		FROM eval_bfcl
		%s
		GROUP BY run_id, model, provider
		ORDER BY timestamp DESC`, modelFilter)
	writeQueryResult(w, r, h.db, query, params)
}

type bfclDetailHandler struct{ db Querier }

func (h *bfclDetailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("run_id")
	if runID == "" {
		axon.WriteError(w, http.StatusBadRequest, "run_id is required")
		return
	}
	query := `
		SELECT
			case_id,
			category,
			pass,
			error,
			expected,
			got,
			duration_ms
		FROM eval_bfcl
		WHERE run_id = {run_id:String}
		ORDER BY category, case_id`
	writeQueryResult(w, r, h.db, query, map[string]string{"run_id": runID})
}

type bfclCompareHandler struct{ db Querier }

func (h *bfclCompareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT
			model,
			category,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE pass = true) AS passed,
			ROUND(COUNT(*) FILTER (WHERE pass = true) * 100.0 / COUNT(*), 1) AS accuracy,
			AVG(duration_ms) AS avg_duration_ms
		FROM eval_bfcl
		WHERE run_id IN (
			SELECT run_id FROM eval_bfcl
			GROUP BY run_id
			ORDER BY MIN(timestamp) DESC
			LIMIT 10
		)
		GROUP BY model, category
		ORDER BY model, category`
	writeQueryResult(w, r, h.db, query, nil)
}

type runsHandler struct{ db Querier }

func (h *runsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT
			run_id,
			MIN(timestamp) AS started_at,
			MAX(timestamp) AS completed_at,
			ARG_MIN(description, timestamp) AS description,
			ARG_MIN(agent_slug, timestamp) AS agent_slug
		FROM events_run
		GROUP BY run_id
		ORDER BY started_at DESC`
	writeQueryResult(w, r, h.db, query, nil)
}

type runSummaryHandler struct{ db Querier }

func (h *runSummaryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("run_id")
	if runID == "" {
		axon.WriteError(w, http.StatusBadRequest, "run_id is required")
		return
	}
	query := `
		SELECT
			{run_id:String} AS run_id,
			(SELECT COUNT(*) FROM events_message WHERE run_id = {run_id:String}) AS messages,
			(SELECT COUNT(*) FROM events_tool_invocation WHERE run_id = {run_id:String}) AS tool_invocations,
			(SELECT COUNT(DISTINCT conversation_id) FROM events_message WHERE run_id = {run_id:String}) AS conversations,
			(SELECT COUNT(*) FROM events_memory WHERE run_id = {run_id:String}) AS memories,
			(SELECT COUNT(*) FROM events_relationship WHERE run_id = {run_id:String}) AS relationship_snapshots,
			(SELECT COUNT(*) FROM events_consolidation WHERE run_id = {run_id:String}) AS consolidations`
	writeQueryResult(w, r, h.db, query, map[string]string{"run_id": runID})
}
