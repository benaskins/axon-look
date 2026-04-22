package duckdb

import (
	"net/http"
	"strconv"

	"github.com/benaskins/axon"
)

// traceByIDHandler returns every span for a given trace_id, ordered by
// start_time ascending so a waterfall renderer gets rows in arrival
// order without client-side sorting.
type traceByIDHandler struct{ db Querier }

func (h *traceByIDHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("trace_id")
	if traceID == "" {
		axon.WriteError(w, http.StatusBadRequest, "trace_id is required")
		return
	}
	query := `
		SELECT trace_id, span_id, parent_span_id, name, kind, service_name,
		       scope_name, scope_version, start_time, end_time, duration_ns,
		       status_code, status_message, attributes, events, links
		FROM spans
		WHERE trace_id = {trace_id:String}
		ORDER BY start_time ASC`
	writeQueryResult(w, r, h.db, query, map[string]string{"trace_id": traceID})
}

// servicesHandler lists distinct services with span counts and the
// most recent activity, so the dashboard can render a service
// directory sorted by activity.
type servicesHandler struct{ db Querier }

func (h *servicesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT
			service_name,
			COUNT(*) AS span_count,
			MAX(start_time) AS last_seen
		FROM spans
		WHERE service_name <> ''
		GROUP BY service_name
		ORDER BY last_seen DESC`
	writeQueryResult(w, r, h.db, query, nil)
}

// spansRecentHandler returns the most recent N spans, optionally
// filtered by service. It's the default landing query for the trace
// explorer: "what's happening right now?"
type spansRecentHandler struct{ db Querier }

func (h *spansRecentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 1000 {
		limit = n
	}

	params := map[string]string{"limit": strconv.Itoa(limit)}
	where := ""
	if service != "" {
		where = "WHERE service_name = {service:String}"
		params["service"] = service
	}
	query := `
		SELECT trace_id, span_id, name, kind, service_name,
		       start_time, duration_ns, status_code
		FROM spans ` + where + `
		ORDER BY start_time DESC
		LIMIT {limit:UInt32}`
	writeQueryResult(w, r, h.db, query, params)
}
