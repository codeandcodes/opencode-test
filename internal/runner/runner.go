// Package runner executes opencode benchmark jobs serially, grouped by model.
package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"opencode-bench/internal/store"
	"opencode-bench/internal/tasks"
)

// ErrBusy is returned when a batch is already running.
var ErrBusy = errors.New("a batch is already running")

const checkTimeout = 5 * time.Minute

// Event is a progress notification streamed to subscribers.
type Event struct {
	Type   string `json:"type"` // job_start | job_end | batch_end
	Task   string `json:"task,omitempty"`
	Model  string `json:"model,omitempty"`
	Status string `json:"status,omitempty"`
	Done   int    `json:"done"`
	Total  int    `json:"total"`
}

type job struct {
	task  tasks.Task
	model string
}

type Runner struct {
	opencode string
	store    *store.Store
	// Timeout maps a task to its execution deadline; overridable in tests.
	Timeout func(tasks.Task) time.Duration
	// Env is the environment for opencode processes; defaults to os.Environ().
	Env []string

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	current Event
	done    int
	total   int
	subs    map[int]chan Event
	nextSub int
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
func (r *Runner) Cancel() {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()
}

// StartBatch queues models×tasks in model-major order and runs them in a
// single background goroutine. ErrBusy if a batch is active.
func (r *Runner) StartBatch(models []string, ts []tasks.Task) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return ErrBusy
	}
	var jobs []job
	for _, m := range models {
		for _, t := range ts {
			jobs = append(jobs, job{task: t, model: m})
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.running, r.cancel = true, cancel
	r.done, r.total = 0, len(jobs)
	r.mu.Unlock()

	go func() {
		defer func() {
			r.mu.Lock()
			r.running = false
			r.cancel = nil
			r.mu.Unlock()
			r.publish(Event{Type: "batch_end"})
		}()
		for _, j := range jobs {
			if ctx.Err() != nil {
				return
			}
			r.publish(Event{Type: "job_start", Task: j.task.ID, Model: j.model, Status: "running"})
			status := r.runJob(ctx, j)
			r.publish(Event{Type: "job_end", Task: j.task.ID, Model: j.model, Status: status})
		}
	}()
	return nil
}

// runJob executes one (task, model) pair and persists its Result.
func (r *Runner) runJob(ctx context.Context, j job) string {
	ref, ws, err := r.store.NewRunDir(j.task.ID, j.model)
	if err != nil {
		return "error"
	}
	res := store.Result{Task: j.task.ID, Model: j.model, Timestamp: ref.Timestamp,
		StartedAt: time.Now().UTC()}
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
		r.store.WriteResult(ref, res)
		return status
	}

	jobCtx, cancel := context.WithTimeout(ctx, r.Timeout(j.task))
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
		"--dir", ws, "-m", "llama-swap/"+j.model, "--format", "json", "--auto", j.task.Prompt)
	cmd.Stdout = eventsFile
	cmd.Stderr = stderrFile
	cmd.Env = r.Env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return finish("error", err.Error())
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	select {
	case <-jobCtx.Done():
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-waitDone
		if ctx.Err() != nil {
			return finish("error", "cancelled")
		}
		return finish("timeout", "task timeout exceeded")
	case err := <-waitDone:
		if err != nil {
			return finish("error", "opencode: "+err.Error())
		}
	}
	if fi, err := eventsFile.Stat(); err != nil || fi.Size() == 0 {
		return finish("error", "opencode produced no events")
	}

	if j.task.Type != "check" {
		return finish("done", "")
	}
	checkCtx, cancelCheck := context.WithTimeout(ctx, checkTimeout)
	defer cancelCheck()
	check := exec.CommandContext(checkCtx, "bash", "-c", j.task.Check)
	check.Dir = ws
	check.Env = r.Env
	out, err := check.CombinedOutput()
	os.WriteFile(filepath.Join(r.store.RunPath(ref), "check.log"), out, 0o644)
	if err != nil {
		return finish("fail", "")
	}
	return finish("pass", "")
}
