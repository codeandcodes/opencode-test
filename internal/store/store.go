// Package store persists benchmark runs as plain files under a root dir:
//
//	runs/<taskID>/<modelID>/<timestamp>/{workspace/,events.jsonl,stderr.log,check.log,result.json}
package store

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Result is the persisted outcome of one run.
type Result struct {
	Task        string    `json:"task"`
	Model       string    `json:"model"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	DurationSec float64   `json:"duration_sec"`
	Messages    int       `json:"messages"`
	ToolCalls   int       `json:"tool_calls"`
	TokensIn    int       `json:"tokens_in"`
	TokensOut   int       `json:"tokens_out"`
	// TokensReasoning counts thinking tokens reported separately from output.
	TokensReasoning int `json:"tokens_reasoning"`
	// CacheRead counts prompt tokens served from the provider's cache.
	CacheRead int `json:"cache_read"`
	// GenSeconds is model-active time from the event stream: step windows
	// minus tool execution. Includes prompt processing; the first step also
	// includes model load.
	GenSeconds float64 `json:"gen_seconds"`
	Error      string  `json:"error,omitempty"`
	Timestamp  string  `json:"timestamp"`
}

// RunRef identifies one run directory.
type RunRef struct {
	Task      string `json:"task"`
	Model     string `json:"model"`
	Timestamp string `json:"timestamp"`
}

// FileEntry is one workspace file or directory, path relative to workspace.
type FileEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Dir  bool   `json:"dir"`
}

type Store struct{ root string }

func New(root string) *Store { return &Store{root: root} }

// timestamp layout: RFC3339 with colons replaced so it is filesystem-safe
// everywhere and still sorts lexicographically by time.
const tsLayout = "2006-01-02T15-04-05Z"

// NewRunDir creates runs/<task>/<model>/<ts>/workspace and returns the ref
// and the absolute workspace path.
func (s *Store) NewRunDir(task, model string) (RunRef, string, error) {
	ref := RunRef{Task: task, Model: model, Timestamp: time.Now().UTC().Format(tsLayout)}
	ws := filepath.Join(s.RunPath(ref), "workspace")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		return RunRef{}, "", err
	}
	return ref, ws, nil
}

func (s *Store) RunPath(ref RunRef) string {
	return filepath.Join(s.root, ref.Task, ref.Model, ref.Timestamp)
}

func (s *Store) WriteResult(ref RunRef, r Result) error {
	out, err := json.MarshalIndent(r, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.RunPath(ref), "result.json"), out, 0o644)
}

func (s *Store) ReadResult(ref RunRef) (Result, error) {
	raw, err := os.ReadFile(filepath.Join(s.RunPath(ref), "result.json"))
	if err != nil {
		return Result{}, err
	}
	var r Result
	if err := json.Unmarshal(raw, &r); err != nil {
		return Result{}, err
	}
	return r, nil
}

// resultOrInterrupted reads a run dir's result, synthesizing an
// "interrupted" result when result.json is absent (crashed/killed server).
func (s *Store) resultOrInterrupted(task, model, ts string) Result {
	ref := RunRef{Task: task, Model: model, Timestamp: ts}
	if r, err := s.ReadResult(ref); err == nil {
		return r
	}
	return Result{Task: task, Model: model, Status: "interrupted", Timestamp: ts}
}

// Latest returns the newest result per (task, model).
func (s *Store) Latest() (map[string]map[string]Result, error) {
	out := map[string]map[string]Result{}
	tasks, err := os.ReadDir(s.root)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if !t.IsDir() {
			continue
		}
		models, err := os.ReadDir(filepath.Join(s.root, t.Name()))
		if err != nil {
			continue
		}
		for _, m := range models {
			if !m.IsDir() {
				continue
			}
			runs, err := os.ReadDir(filepath.Join(s.root, t.Name(), m.Name()))
			if err != nil || len(runs) == 0 {
				continue
			}
			names := make([]string, 0, len(runs))
			for _, r := range runs {
				if r.IsDir() {
					names = append(names, r.Name())
				}
			}
			if len(names) == 0 {
				continue
			}
			sort.Strings(names) // tsLayout sorts chronologically
			latest := names[len(names)-1]
			if out[t.Name()] == nil {
				out[t.Name()] = map[string]Result{}
			}
			out[t.Name()][m.Name()] = s.resultOrInterrupted(t.Name(), m.Name(), latest)
		}
	}
	return out, nil
}

// History returns all results for a (task, model) cell, newest first.
func (s *Store) History(task, model string) ([]Result, error) {
	dir := filepath.Join(s.root, task, model)
	runs, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []Result{}, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, r := range runs {
		if r.IsDir() {
			names = append(names, r.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	out := make([]Result, 0, len(names))
	for _, ts := range names {
		out = append(out, s.resultOrInterrupted(task, model, ts))
	}
	return out, nil
}

// SafePath resolves rel inside the run's workspace, rejecting escapes.
func (s *Store) SafePath(ref RunRef, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	ws := filepath.Join(s.RunPath(ref), "workspace")
	p := filepath.Clean(filepath.Join(ws, rel))
	r, err := filepath.Rel(ws, p)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return p, nil
}

// ListFiles walks the workspace, returning sorted workspace-relative entries.
func (s *Store) ListFiles(ref RunRef) ([]FileEntry, error) {
	ws := filepath.Join(s.RunPath(ref), "workspace")
	var out []FileEntry
	err := filepath.WalkDir(ws, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == ws {
			return nil
		}
		rel, err := filepath.Rel(ws, p)
		if err != nil {
			return err
		}
		e := FileEntry{Path: filepath.ToSlash(rel), Dir: d.IsDir()}
		if !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				e.Size = fi.Size()
			}
		}
		out = append(out, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
