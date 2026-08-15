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
	"sort"
	"time"

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
	s.mux.HandleFunc("GET /api/runs/active/tail", s.handleActiveTail)
	s.mux.HandleFunc("GET /api/runs/resumable", s.handleResumableGet)
	s.mux.HandleFunc("POST /api/runs/resume", s.handleResume)
	s.mux.HandleFunc("DELETE /api/runs/resumable", s.handleResumableDismiss)
	s.mux.HandleFunc("POST /api/runs/{task}/{model}/{ts}/verdict", s.handleVerdictSet)
	s.mux.HandleFunc("DELETE /api/runs/{task}/{model}/{ts}/verdict", s.handleVerdictClear)
	s.mux.HandleFunc("GET /api/leaderboard", s.handleLeaderboard)
	s.mux.HandleFunc("GET /api/events", s.handleSSE)
}

// leaderboardRow aggregates one model's standing across all task cells.
type leaderboardRow struct {
	Model            string  `json:"model"`
	CheckCells       int     `json:"check_cells"`        // check cells with ≥1 completed sample
	CheckCellsPassed int     `json:"check_cells_passed"` // of those, cells with ≥1 pass
	ReviewsDone      int     `json:"reviews_done"`       // review cells with ≥1 completed sample
	VerdictGood      int     `json:"verdict_good"`
	VerdictBad       int     `json:"verdict_bad"`
	Errors           int     `json:"errors"` // cells whose latest run is an infrastructure failure
	MedianTps        float64 `json:"median_tps"`
	Samples          int     `json:"samples"`
}

func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	latest, err := s.cfg.Store.Latest()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	rows := map[string]*leaderboardRow{}
	var tpsSamples = map[string][]float64{}
	for taskID, byModel := range latest {
		for model, latestRes := range byModel {
			row := rows[model]
			if row == nil {
				row = &leaderboardRow{Model: model}
				rows[model] = row
			}
			history, err := s.cfg.Store.History(taskID, model)
			if err != nil {
				continue
			}
			agg := store.Aggregate(history)
			row.Samples += agg.Samples
			row.VerdictGood += agg.VerdictGood
			row.VerdictBad += agg.VerdictBad
			if agg.MedianTps > 0 {
				tpsSamples[model] = append(tpsSamples[model], agg.MedianTps)
			}
			// Task type is inferred from statuses: pass/fail only come
			// from check tasks, done only from review tasks.
			if agg.Passes+agg.Fails > 0 {
				row.CheckCells++
				if agg.Passes > 0 {
					row.CheckCellsPassed++
				}
			} else if agg.Dones > 0 {
				row.ReviewsDone++
			}
			switch latestRes.Status {
			case "error", "timeout", "interrupted":
				row.Errors++
			}
		}
	}
	out := make([]leaderboardRow, 0, len(rows))
	for model, row := range rows {
		if tps := tpsSamples[model]; len(tps) > 0 {
			sort.Float64s(tps)
			mid := len(tps) / 2
			if len(tps)%2 == 1 {
				row.MedianTps = tps[mid]
			} else {
				row.MedianTps = (tps[mid-1] + tps[mid]) / 2
			}
		}
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CheckCellsPassed != out[j].CheckCellsPassed {
			return out[i].CheckCellsPassed > out[j].CheckCellsPassed
		}
		return out[i].Model < out[j].Model
	})
	writeJSON(w, http.StatusOK, out)
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
		// Force queues exactly Samples new runs per cell regardless of
		// history; without it, cells are topped up to Samples completed
		// measurements (default 1, i.e. skip already-completed cells).
		Force   bool `json:"force"`
		Samples int  `json:"samples"`
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
	if req.Samples <= 0 {
		req.Samples = 1
	}
	if req.Samples > 10 {
		writeErr(w, http.StatusBadRequest, errors.New("samples must be <= 10"))
		return
	}
	pairs := runner.Pairs(req.Models, selected)
	var jobs []runner.JobSpec
	skipped := 0
	for _, j := range pairs {
		want := req.Samples
		if !req.Force {
			completed, err := s.completedCount(j)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err)
				return
			}
			want -= completed
		}
		if want <= 0 {
			skipped++
			continue
		}
		for range want {
			jobs = append(jobs, j)
		}
	}
	if len(jobs) == 0 {
		writeJSON(w, http.StatusOK, map[string]int{"jobs": 0, "skipped": skipped})
		return
	}
	if err := s.cfg.Runner.StartBatch(jobs); err != nil {
		if errors.Is(err, runner.ErrBusy) {
			writeErr(w, http.StatusConflict, err)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]int{"jobs": len(jobs), "skipped": skipped})
}

// handleActiveTail reports live progress of the currently executing job:
// identity, event-stream stats, staleness of the last event, and the most
// recent raw events for a live transcript.
func (s *Server) handleActiveTail(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.cfg.Runner.CurrentRef()
	if !ok {
		writeErr(w, http.StatusNotFound, errors.New("no job running"))
		return
	}
	eventsPath := filepath.Join(s.cfg.Store.RunPath(ref), "events.jsonl")
	stats := runner.ParseEvents(eventsPath)
	recent := runner.TailEvents(eventsPath, 30)
	if recent == nil {
		recent = []json.RawMessage{}
	}
	ageSec := -1.0
	if len(recent) > 0 {
		var last struct {
			Timestamp float64 `json:"timestamp"`
		}
		for i := len(recent) - 1; i >= 0; i-- {
			if json.Unmarshal(recent[i], &last) == nil && last.Timestamp > 0 {
				ageSec = float64(time.Now().UnixMilli())/1000 - last.Timestamp/1000
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"task":               ref.Task,
		"model":              ref.Model,
		"timestamp":          ref.Timestamp,
		"steps":              stats.Messages,
		"tool_calls":         stats.ToolCalls,
		"tokens_out":         stats.TokensOut + stats.TokensReasoning,
		"last_event_age_sec": ageSec,
		"recent":             recent,
	})
}

// completedCount reports how many completed measurements (done/pass/fail)
// a cell's history holds.
func (s *Server) completedCount(j runner.JobSpec) (int, error) {
	history, err := s.cfg.Store.History(j.Task.ID, j.Model)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, res := range history {
		if res.Status == "done" || res.Status == "pass" || res.Status == "fail" {
			n++
		}
	}
	return n, nil
}

// filterCompleted drops jobs whose cell already holds a completed
// measurement (done/pass/fail) anywhere in its history.
func (s *Server) filterCompleted(jobs []runner.JobSpec) ([]runner.JobSpec, int, error) {
	kept := jobs[:0]
	skipped := 0
	for _, j := range jobs {
		completed, err := s.completedCount(j)
		if err != nil {
			return nil, 0, err
		}
		if completed > 0 {
			skipped++
			continue
		}
		kept = append(kept, j)
	}
	return kept, skipped, nil
}

func (s *Server) readBatchState() (runner.BatchState, error) {
	var st runner.BatchState
	if s.cfg.BatchStateFile == "" {
		return st, os.ErrNotExist
	}
	raw, err := os.ReadFile(s.cfg.BatchStateFile)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return st, err
	}
	return st, nil
}

func (s *Server) handleResumableGet(w http.ResponseWriter, r *http.Request) {
	running, _, _, _ := s.cfg.Runner.Active()
	st, err := s.readBatchState()
	if err != nil || running || len(st.Jobs) == 0 {
		writeErr(w, http.StatusNotFound, errors.New("no resumable batch"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": st.Jobs, "count": len(st.Jobs)})
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	st, err := s.readBatchState()
	if err != nil || len(st.Jobs) == 0 {
		writeErr(w, http.StatusNotFound, errors.New("no resumable batch"))
		return
	}
	lib, err := tasks.Load(s.cfg.TasksDir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	var jobs []runner.JobSpec
	dropped := 0
	for _, bj := range st.Jobs {
		t, ok := lib.Get(bj.Task)
		if !ok {
			dropped++
			continue
		}
		jobs = append(jobs, runner.JobSpec{Model: bj.Model, Task: t})
	}
	jobs, skipped, err := s.filterCompleted(jobs)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if len(jobs) == 0 {
		os.Remove(s.cfg.BatchStateFile)
		writeJSON(w, http.StatusOK, map[string]int{"jobs": 0, "skipped": skipped, "dropped": dropped})
		return
	}
	if err := s.cfg.Runner.StartBatch(jobs); err != nil {
		if errors.Is(err, runner.ErrBusy) {
			writeErr(w, http.StatusConflict, err)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// StartBatch rewrote the state file for the new batch; the old one is superseded.
	writeJSON(w, http.StatusAccepted, map[string]int{"jobs": len(jobs), "skipped": skipped, "dropped": dropped})
}

func (s *Server) handleResumableDismiss(w http.ResponseWriter, r *http.Request) {
	if s.cfg.BatchStateFile != "" {
		os.Remove(s.cfg.BatchStateFile)
	}
	w.WriteHeader(http.StatusNoContent)
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
	agg := map[string]map[string]store.CellAgg{}
	for taskID, byModel := range m {
		agg[taskID] = map[string]store.CellAgg{}
		for model := range byModel {
			if history, err := s.cfg.Store.History(taskID, model); err == nil {
				agg[taskID][model] = store.Aggregate(history)
			}
		}
	}
	running, cur, done, total := s.cfg.Runner.Active()
	writeJSON(w, http.StatusOK, map[string]any{
		"matrix": m,
		"agg":    agg,
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
