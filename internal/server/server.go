// Package server wires the opencode-bench HTTP API and embedded UI.
package server

import (
	"net/http"
)

// Config carries the paths and collaborators the server needs.
type Config struct {
	OpencodeConfigPath string
	TasksDir           string
	RunsDir            string
	OpencodeBin        string
}

type Server struct {
	cfg Config
	mux *http.ServeMux
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }
