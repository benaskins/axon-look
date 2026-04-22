package duckdb

import (
	"net/http"

	"github.com/benaskins/axon"
)

// scaffoldsHandler lists distinct scaffold.id values seen across span
// resource attributes, with span counts, first/last timestamps, and the
// services that emitted them. Spans without a scaffold.id attribute are
// excluded so the view stays focused on factory runs.
type scaffoldsHandler struct{ db Querier }

func (h *scaffoldsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT
			json_extract_string(resource_attributes, '$."scaffold.id"') AS scaffold_id,
			COUNT(*) AS span_count,
			MIN(start_time) AS first_seen,
			MAX(start_time) AS last_seen,
			string_agg(DISTINCT service_name, ',' ORDER BY service_name) AS services
		FROM spans
		WHERE json_extract_string(resource_attributes, '$."scaffold.id"') IS NOT NULL
		GROUP BY scaffold_id
		ORDER BY last_seen DESC`
	writeQueryResult(w, r, h.db, query, nil)
}

// scaffoldSpansHandler returns every span belonging to a given
// scaffold.id, ordered by start_time ascending so a waterfall renderer
// gets rows in factory-run order.
type scaffoldSpansHandler struct{ db Querier }

func (h *scaffoldSpansHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	scaffoldID := r.PathValue("scaffold_id")
	if scaffoldID == "" {
		axon.WriteError(w, http.StatusBadRequest, "scaffold_id is required")
		return
	}
	query := `
		SELECT trace_id, span_id, parent_span_id, name, kind, service_name,
		       start_time, end_time, duration_ns, status_code,
		       resource_attributes, attributes
		FROM spans
		WHERE json_extract_string(resource_attributes, '$."scaffold.id"') = {scaffold_id:String}
		ORDER BY start_time ASC`
	writeQueryResult(w, r, h.db, query, map[string]string{"scaffold_id": scaffoldID})
}
