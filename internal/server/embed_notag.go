//go:build !ui

package server

import "net/http"

// registerUI is the no-embed fallback: point users at the full build.
func (s *Server) registerUI() {
	s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "ui not embedded; build with make build",
		})
	})
}
