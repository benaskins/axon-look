package anal

import (
	"net/http"

	"github.com/benaskins/axon"
)

// Server is the analytics service HTTP server.
type Server struct {
	mux *http.ServeMux
}

// NewServer creates an analytics server.
func NewServer() *Server {
	return &Server{}
}

// Handler returns the HTTP handler with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		axon.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux = mux
	return mux
}
