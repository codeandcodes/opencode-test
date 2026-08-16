// Package runner executes opencode benchmark jobs serially, grouped by model.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"opencode-bench/internal/store"
	"opencode-bench/internal/tasks"
)

// ErrBusy is returned when a batch is already running.
var ErrBusy = errors.New("a batch is already running")

const defaultCheckTimeout = 5 * time.Minute

// Event is a progress notification streamed to subscribers.
type Event struct {
	Type   string `json:"type"` // job_start | job_end | batch_end
	Task   string `json:"task,omitempty"`
	Model  string `json:"model,omitempty"`
	Status string `json:"status,omitempty"`
	Done   int    `json:"done"`
	Total  int    `json:"total"`
}

// JobSpec names one (model, task) pair to execute.
type JobSpec struct {
	Model string
	Task  tasks.Task
}

// Pairs expands models×tasks in model-major order, so all of one model's
// tasks run before the next model loads (amortizing the swap cost).
func Pairs(models []string, ts []tasks.Task) []JobSpec {
	var out []JobSpec
	for _, m := range models {
		for _, t := range ts {
			out = append(out, JobSpec{Model: m, Task: t})
		}
	}
	return out
}

type Runner struct {
	opencode string
	store    *store.Store
	// Timeout maps a task to its execution deadline; overridable in tests.
	Timeout func(tasks.Task) time.Duration
	// Env is the environment for opencode processes; defaults to os.Environ().
	Env []string
	// LlamaSwapConfig optionally points at llama-swap's YAML config so each
	// run can snapshot the model's serving entry into its provenance.
	LlamaSwapConfig string
	// StateFile optionally persists the pending job list so an interrupted
	// batch can be resumed after a crash or restart.
	StateFile string
	// IdleTimeout kills a job whose event stream has been silent this long
	// (0 disables). Event parts only land when they complete, so legitimate
	// long generations produce minutes of silence — keep this generous.
	IdleTimeout time.Duration
	// CheckTimeout bounds a check script's execution (default 5 minutes).
	CheckTimeout time.Duration

	mu         sync.Mutex
	running    bool
	cancel     context.CancelFunc
	shutdown   bool // true when the current cancel is a shutdown, not a user cancel
	current    Event
	currentRef store.RunRef
	haveRef    bool
	done       int
	total      int
	subs       map[int]chan Event
	nextSub    int
}

// CurrentRef returns the run directory reference of the job executing right
// now, if any.
func (r *Runner) CurrentRef() (store.RunRef, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running || !r.haveRef {
		return store.RunRef{}, false
	}
	return r.currentRef, true
}

func (r *Runner) setCurrentRef(ref store.RunRef) {
	r.mu.Lock()
	r.currentRef, r.haveRef = ref, true
	r.mu.Unlock()
}

// BatchState is the on-disk format of StateFile: jobs not yet completed,
// in execution order (the first entry may be a killed in-flight job).
type BatchState struct {
	Jobs []BatchJob `json:"jobs"`
}

type BatchJob struct {
	Model string `json:"model"`
	Task  string `json:"task"`
}

func (r *Runner) writeState(jobs []JobSpec) {
	if r.StateFile == "" {
		return
	}
	st := BatchState{Jobs: []BatchJob{}}
	for _, j := range jobs {
		st.Jobs = append(st.Jobs, BatchJob{Model: j.Model, Task: j.Task.ID})
	}
	if out, err := json.Marshal(st); err == nil {
		os.WriteFile(r.StateFile, out, 0o644)
	}
}

func (r *Runner) clearState() {
	if r.StateFile != "" {
		os.Remove(r.StateFile)
	}
}

// modelArg maps a model ID to opencode's provider/model form. Bare IDs are
// llama-swap models; IDs already containing a provider pass through.
func modelArg(model string) string {
	if strings.Contains(model, "/") {
		return model
	}
	return "llama-swap/" + model
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b && a > 0 {
		return a
	}
	return b
}

func New(opencodeBin string, st *store.Store) *Runner {
	return &Runner{
		opencode: opencodeBin,
		store:    st,
		Timeout:  func(t tasks.Task) time.Duration { return time.Duration(t.TimeoutMinutes) * time.Minute },
		Env:      os.Environ(),
		subs:     map[int]chan Event{},
	}
}

// Subscribe returns a channel of progress events and an unsubscribe func.
// Slow consumers drop events rather than blocking the runner.
func (r *Runner) Subscribe() (<-chan Event, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.nextSub
	r.nextSub++
	ch := make(chan Event, 64)
	r.subs[id] = ch
	return ch, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.subs, id)
	}
}

func (r *Runner) publish(e Event) {
	r.mu.Lock()
	r.current = e
	if e.Type == "job_end" {
		r.done++
	}
	e.Done, e.Total = r.done, r.total
	for _, ch := range r.subs {
		select {
		case ch <- e:
		default: // drop for slow consumers
		}
	}
	r.mu.Unlock()
}

// Active reports whether a batch is running and its progress.
func (r *Runner) Active() (bool, Event, int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running, r.current, r.done, r.total
}

// Cancel kills the current job's process group and skips remaining jobs.
// The persisted batch state is discarded — cancelling is deliberate.
func (r *Runner) Cancel() {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()
}

// Shutdown cancels like Cancel but preserves the persisted batch state so
// the interrupted batch can be offered for resume after restart.
func (r *Runner) Shutdown() {
	r.mu.Lock()
	if r.cancel != nil {
		r.shutdown = true
		r.cancel()
	}
	r.mu.Unlock()
}

// StartBatch queues the given jobs (see Pairs for the standard expansion)
// and runs them serially in a single background goroutine. ErrBusy if a
// batch is active.
func (r *Runner) StartBatch(jobs []JobSpec) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.running, r.cancel = true, cancel
	r.shutdown = false
	r.done, r.total = 0, len(jobs)
	r.mu.Unlock()
	r.writeState(jobs)

	go func() {
		defer func() {
			r.mu.Lock()
			r.running = false
			r.cancel = nil
			r.haveRef = false
			preserve := r.shutdown
			r.mu.Unlock()
			if !preserve {
				r.clearState()
			}
			r.publish(Event{Type: "batch_end"})
		}()
		for i, j := range jobs {
			if ctx.Err() != nil {
				return
			}
			r.writeState(jobs[i:]) // pending = current job + the rest
			r.publish(Event{Type: "job_start", Task: j.Task.ID, Model: j.Model, Status: "running"})
			status := r.runJob(ctx, j)
			r.publish(Event{Type: "job_end", Task: j.Task.ID, Model: j.Model, Status: status})
		}
	}()
	return nil
}

// runJob executes one (task, model) pair and persists its Result.
func (r *Runner) runJob(ctx context.Context, j JobSpec) string {
	ref, ws, err := r.store.NewRunDir(j.Task.ID, j.Model)
	if err != nil {
		return "error"
	}
	r.setCurrentRef(ref)
	// Anchor the agent's project-root discovery at the workspace: without
	// its own .git, opencode walks up and finds the harness repo, and
	// models then write into the repo root via absolute "project" paths.
	gitInit := exec.Command("git", "init", "-q")
	gitInit.Dir = ws
	gitInit.Run() // best-effort
	prov := buildProvenance(j.Task, j.Model, r.LlamaSwapConfig)
	writeProvenance(filepath.Join(r.store.RunPath(ref), "provenance.json"), prov)
	res := store.Result{Task: j.Task.ID, Model: j.Model, Timestamp: ref.Timestamp,
		StartedAt: time.Now().UTC(), PromptSHA: prov.PromptSHA}
	finish := func(status, errMsg string) string {
		res.Status = status
		res.Error = errMsg
		res.FinishedAt = time.Now().UTC()
		res.DurationSec = res.FinishedAt.Sub(res.StartedAt).Seconds()
		stats := ParseEvents(filepath.Join(r.store.RunPath(ref), "events.jsonl"))
		res.Messages, res.ToolCalls = stats.Messages, stats.ToolCalls
		res.TokensIn, res.TokensOut = stats.TokensIn, stats.TokensOut
		res.TokensReasoning, res.CacheRead = stats.TokensReasoning, stats.CacheRead
		res.GenSeconds = stats.GenSeconds
		res.LoadSeconds = stats.LoadSeconds
		r.store.WriteResult(ref, res)
		return status
	}

	jobCtx, cancel := context.WithTimeout(ctx, r.Timeout(j.Task))
	defer cancel()

	eventsFile, err := os.Create(filepath.Join(r.store.RunPath(ref), "events.jsonl"))
	if err != nil {
		return finish("error", err.Error())
	}
	defer eventsFile.Close()
	stderrFile, err := os.Create(filepath.Join(r.store.RunPath(ref), "stderr.log"))
	if err != nil {
		return finish("error", err.Error())
	}
	defer stderrFile.Close()

	cmd := exec.Command(r.opencode, "run",
		"--dir", ws, "-m", modelArg(j.Model), "--format", "json", "--auto", j.Task.Prompt)
	cmd.Stdout = eventsFile
	cmd.Stderr = stderrFile
	cmd.Env = r.Env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return finish("error", err.Error())
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	eventsPath := filepath.Join(r.store.RunPath(ref), "events.jsonl")
	var idleTick <-chan time.Time
	if r.IdleTimeout > 0 {
		t := time.NewTicker(minDuration(r.IdleTimeout/4, 5*time.Second))
		defer t.Stop()
		idleTick = t.C
	}
	killAndDrain := func() {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-waitDone
	}
wait:
	for {
		select {
		case <-jobCtx.Done():
			killAndDrain()
			if ctx.Err() != nil {
				return finish("error", "cancelled")
			}
			return finish("timeout", "task timeout exceeded")
		case <-idleTick:
			if fi, err := os.Stat(eventsPath); err == nil && time.Since(fi.ModTime()) > r.IdleTimeout {
				killAndDrain()
				return finish("timeout", fmt.Sprintf("idle timeout: no events for %s", r.IdleTimeout))
			}
		case err := <-waitDone:
			if err != nil {
				return finish("error", "opencode: "+err.Error())
			}
			break wait
		}
	}
	if fi, err := eventsFile.Stat(); err != nil || fi.Size() == 0 {
		return finish("error", "opencode produced no events")
	}

	if j.Task.Type != "check" {
		return finish("done", "")
	}
	status, errMsg := r.runCheck(ctx, j, ref, ws)
	if raw, err := os.ReadFile(filepath.Join(r.store.RunPath(ref), "check.log")); err == nil {
		res.CheckPassed, res.CheckFailed, res.CheckParsed = ParseCheckLog(string(raw))
	}
	return finish(status, errMsg)
}

// runCheck executes a check script in its own process group with output
// captured straight to check.log. Files, not pipes: a backgrounded child
// inheriting the script's fds must never be able to block us, and on
// timeout the whole group is killed.
func (r *Runner) runCheck(ctx context.Context, j JobSpec, ref store.RunRef, ws string) (string, string) {
	timeout := r.CheckTimeout
	if timeout <= 0 {
		timeout = defaultCheckTimeout
	}
	logFile, err := os.Create(filepath.Join(r.store.RunPath(ref), "check.log"))
	if err != nil {
		return "error", "check log: " + err.Error()
	}
	defer logFile.Close()

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	check := exec.Command("bash", "-c", j.Task.Check)
	check.Dir = ws
	check.Env = r.Env
	check.Stdout = logFile
	check.Stderr = logFile
	check.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := check.Start(); err != nil {
		return "error", "check start: " + err.Error()
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- check.Wait() }()
	select {
	case <-checkCtx.Done():
		syscall.Kill(-check.Process.Pid, syscall.SIGKILL)
		<-waitDone
		if ctx.Err() != nil {
			return "error", "cancelled"
		}
		return "fail", fmt.Sprintf("check timeout after %s", timeout)
	case err := <-waitDone:
		if err != nil {
			return "fail", ""
		}
	}
	return "pass", ""
}
