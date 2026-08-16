// Package server wires the opencode-bench HTTP API and embedded UI.
package server

import (
	"net/http"

	"opencode-bench/internal/runner"
	"opencode-bench/internal/store"
)

// Config carries the paths and collaborators the server needs.
type Config struct {
	OpencodeConfigPath string
	TasksDir           string
	RunsDir            string
	OpencodeBin        string
	// BatchStateFile is where the runner persists its pending job list;
	// the server reads it to offer resume of interrupted batches.
	BatchStateFile string
	// ExtraModels are provider-qualified reference models (e.g. a free
	// hosted model) benchmarked alongside the llama-swap fleet. Each entry
	// is "provider/model" or "provider/model=Display Name".
	ExtraModels []string
	Store       *store.Store
	Runner      *runner.Runner
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
	if cfg.Store != nil && cfg.Runner != nil {
		s.registerAPI()
	}
	s.registerUI()
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }
