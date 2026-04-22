package duckdb

import (
	"embed"
	"net/http"

	"github.com/benaskins/axon"
)

// Server is the DuckDB-backed analytics service HTTP server. Its route set
// mirrors the root ClickHouse server so the SvelteKit dashboard can point
// at either backend without changes.
type Server struct {
	mux         *http.ServeMux
	db          *DuckDB
	staticFiles *embed.FS
}

// NewServer creates a DuckDB-backed analytics server. If db is nil, only
// /health and the static SPA fallback are wired.
func NewServer(staticFiles *embed.FS, db *DuckDB) *Server {
	return &Server{staticFiles: staticFiles, db: db}
}

// Handler returns the HTTP handler with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		axon.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	if s.db != nil {
		mux.Handle("POST /api/events", &ingestHandler{db: s.db})
		mux.Handle("GET /api/agents/{slug}/stats", &statsHandler{db: s.db})
		mux.Handle("GET /api/agents/{slug}/messages", &messagesHandler{db: s.db})
		mux.Handle("GET /api/agents/{slug}/tools", &toolsHandler{db: s.db})
		mux.Handle("GET /api/agents/{slug}/relationship", &relationshipHandler{db: s.db})
		mux.Handle("GET /api/agents/{slug}/memories", &memoriesHandler{db: s.db})
		mux.Handle("GET /api/agents/{slug}/conversations", &conversationsHandler{db: s.db})
		mux.Handle("GET /api/evals", &evalsListHandler{db: s.db})
		mux.Handle("GET /api/evals/bfcl", &bfclRunsHandler{db: s.db})
		mux.Handle("GET /api/evals/bfcl/compare", &bfclCompareHandler{db: s.db})
		mux.Handle("GET /api/evals/bfcl/{run_id}", &bfclDetailHandler{db: s.db})
		mux.Handle("GET /api/evals/{run_id}", &evalsDetailHandler{db: s.db})
		mux.Handle("GET /api/runs", &runsHandler{db: s.db})
		mux.Handle("GET /api/runs/{run_id}/summary", &runSummaryHandler{db: s.db})
		mux.Handle("GET /api/traces/{trace_id}", &traceByIDHandler{db: s.db})
		mux.Handle("GET /api/services", &servicesHandler{db: s.db})
		mux.Handle("GET /api/spans/recent", &spansRecentHandler{db: s.db})
	}

	if s.staticFiles != nil {
		mux.Handle("/", axon.SPAHandler(*s.staticFiles, "static"))
	}

	s.mux = mux
	return mux
}
