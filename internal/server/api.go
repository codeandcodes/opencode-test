package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"opencode-bench/internal/config"
	"opencode-bench/internal/runner"
	"opencode-bench/internal/store"
	"opencode-bench/internal/tasks"
)

// previewCSP allows a previewed app to use its own files, inline code, and
// data/blob URIs, while blocking all external network access.
const previewCSP = "default-src 'self' 'unsafe-inline' 'unsafe-eval' data: blob:"

const maxDetailEvents = 2000

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func (s *Server) registerAPI() {
	s.mux.HandleFunc("GET /api/models", s.handleModels)
	s.mux.HandleFunc("GET /api/tasks", s.handleTasksList)
	s.mux.HandleFunc("GET /api/tasks/{id}", s.handleTaskDetail)
	s.mux.HandleFunc("POST /api/tasks", s.handleTaskCreate)
	s.mux.HandleFunc("POST /api/runs", s.handleRunsStart)
	s.mux.HandleFunc("DELETE /api/runs/active", s.handleRunsCancel)
	s.mux.HandleFunc("GET /api/runs", s.handleMatrix)
	s.mux.HandleFunc("GET /api/runs/{task}/{model}", s.handleHistory)
	s.mux.HandleFunc("GET /api/runs/{task}/{model}/{ts}", s.handleRunDetail)
	s.mux.HandleFunc("GET /api/runs/{task}/{model}/{ts}/files", s.handleFilesList)
	s.mux.HandleFunc("GET /api/runs/{task}/{model}/{ts}/files/{path...}", s.handleFile(false))
	s.mux.HandleFunc("GET /api/runs/{task}/{model}/{ts}/preview/{path...}", s.handleFile(true))
	s.mux.HandleFunc("POST /api/runs/{task}/{model}/{ts}/verdict", s.handleVerdictSet)
	s.mux.HandleFunc("DELETE /api/runs/{task}/{model}/{ts}/verdict", s.handleVerdictClear)
	s.mux.HandleFunc("GET /api/events", s.handleSSE)
}

func (s *Server) handleVerdictSet(w http.ResponseWriter, r *http.Request) {
	ref := s.ref(r)
	if _, err := s.cfg.Store.ReadResult(ref); err != nil {
		writeErr(w, http.StatusNotFound, errors.New("run not found"))
		return
	}
	var v store.Verdict
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.cfg.Store.WriteVerdict(ref, v); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleVerdictClear(w http.ResponseWriter, r *http.Request) {
	if err := s.cfg.Store.ClearVerdict(s.ref(r)); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	models, err := config.DiscoverModels(s.cfg.OpencodeConfigPath)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusOK, models)
}

func (s *Server) handleTasksList(w http.ResponseWriter, r *http.Request) {
	lib, err := tasks.Load(s.cfg.TasksDir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	type summary struct {
		ID             string `json:"id"`
		Title          string `json:"title"`
		Category       string `json:"category"`
		Type           string `json:"type"`
		TimeoutMinutes int    `json:"timeout_minutes"`
	}
	sums := make([]summary, 0, len(lib.Tasks))
	for _, t := range lib.Tasks {
		sums = append(sums, summary{t.ID, t.Title, t.Category, t.Type, t.TimeoutMinutes})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": sums, "warnings": lib.Warnings})
}

func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	lib, err := tasks.Load(s.cfg.TasksDir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	t, ok := lib.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, errors.New("unknown task"))
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleTaskCreate(w http.ResponseWriter, r *http.Request) {
	var t tasks.Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := tasks.Save(s.cfg.TasksDir, t); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleRunsStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Models []string `json:"models"`
		Tasks  []string `json:"tasks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Models) == 0 || len(req.Tasks) == 0 {
		writeErr(w, http.StatusBadRequest, errors.New("models and tasks must be non-empty"))
		return
	}
	models, err := config.DiscoverModels(s.cfg.OpencodeConfigPath)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, err)
		return
	}
	known := map[string]bool{}
	for _, m := range models {
		known[m.ID] = true
	}
	for _, m := range req.Models {
		if !known[m] {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("unknown model %q", m))
			return
		}
	}
	lib, err := tasks.Load(s.cfg.TasksDir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	var selected []tasks.Task
	for _, id := range req.Tasks {
		t, ok := lib.Get(id)
		if !ok {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("unknown task %q", id))
			return
		}
		selected = append(selected, t)
	}
	if err := s.cfg.Runner.StartBatch(req.Models, selected); err != nil {
		if errors.Is(err, runner.ErrBusy) {
			writeErr(w, http.StatusConflict, err)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]int{"jobs": len(req.Models) * len(selected)})
}

func (s *Server) handleRunsCancel(w http.ResponseWriter, r *http.Request) {
	s.cfg.Runner.Cancel()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMatrix(w http.ResponseWriter, r *http.Request) {
	m, err := s.cfg.Store.Latest()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// Mark results whose task prompt changed since they ran.
	if lib, err := tasks.Load(s.cfg.TasksDir); err == nil {
		current := map[string]string{}
		for _, t := range lib.Tasks {
			current[t.ID] = runner.PromptSHA(t.Prompt)
		}
		for taskID, byModel := range m {
			want, known := current[taskID]
			for model, res := range byModel {
				if known && res.PromptSHA != "" && res.PromptSHA != want {
					res.Stale = true
					byModel[model] = res
				}
			}
		}
	}
	running, cur, done, total := s.cfg.Runner.Active()
	writeJSON(w, http.StatusOK, map[string]any{
		"matrix": m,
		"active": map[string]any{
			"running": running, "task": cur.Task, "model": cur.Model,
			"done": done, "total": total,
		},
	})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	h, err := s.cfg.Store.History(r.PathValue("task"), r.PathValue("model"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func (s *Server) ref(r *http.Request) store.RunRef {
	return store.RunRef{Task: r.PathValue("task"), Model: r.PathValue("model"), Timestamp: r.PathValue("ts")}
}

func (s *Server) handleRunDetail(w http.ResponseWriter, r *http.Request) {
	ref := s.ref(r)
	res, err := s.cfg.Store.ReadResultFull(ref)
	if err != nil {
		writeErr(w, http.StatusNotFound, errors.New("run not found"))
		return
	}
	events := []json.RawMessage{}
	if f, err := os.Open(filepath.Join(s.cfg.Store.RunPath(ref), "events.jsonl")); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
		for sc.Scan() && len(events) < maxDetailEvents {
			line := make([]byte, len(sc.Bytes()))
			copy(line, sc.Bytes())
			if json.Valid(line) {
				events = append(events, line)
			}
		}
		f.Close()
	}
	checkLog := ""
	if raw, err := os.ReadFile(filepath.Join(s.cfg.Store.RunPath(ref), "check.log")); err == nil {
		checkLog = string(raw)
	}
	var provenance json.RawMessage
	if raw, err := os.ReadFile(filepath.Join(s.cfg.Store.RunPath(ref), "provenance.json")); err == nil && json.Valid(raw) {
		provenance = raw
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"result": res, "events": events, "check_log": checkLog, "provenance": provenance,
	})
}

func (s *Server) handleFilesList(w http.ResponseWriter, r *http.Request) {
	files, err := s.cfg.Store.ListFiles(s.ref(r))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) handleFile(preview bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := s.cfg.Store.SafePath(s.ref(r), r.PathValue("path"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			writeErr(w, http.StatusNotFound, errors.New("file not found"))
			return
		}
		ct := mime.TypeByExtension(filepath.Ext(p))
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		if preview {
			w.Header().Set("Content-Security-Policy", previewCSP)
		}
		w.Write(raw)
	}
}
