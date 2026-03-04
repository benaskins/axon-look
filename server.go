package anal

import (
	"net/http"

	"github.com/benaskins/axon"
)

// Server is the analytics service HTTP server.
type Server struct {
	mux *http.ServeMux
	ch  *ClickHouse
}

// NewServer creates an analytics server.
func NewServer(ch ...*ClickHouse) *Server {
	s := &Server{}
	if len(ch) > 0 {
		s.ch = ch[0]
	}
	return s
}

// Handler returns the HTTP handler with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		axon.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	if s.ch != nil {
		mux.Handle("POST /api/events", &ingestHandler{db: s.ch})
	}

	s.mux = mux
	return mux
}
