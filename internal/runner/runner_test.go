package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"opencode-bench/internal/store"
	"opencode-bench/internal/tasks"
)

func stubPath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("testdata/opencode-stub.sh")
	if err != nil {
		t.Fatal(err)
	}
	os.Chmod(p, 0o755)
	return p
}

func newTestRunner(t *testing.T, mode string) (*Runner, *store.Store) {
	t.Helper()
	st := store.New(t.TempDir())
	r := New(stubPath(t), st)
	r.Timeout = func(tasks.Task) time.Duration { return 2 * time.Second }
	r.Env = append(os.Environ(), "STUB_MODE="+mode)
	return r, st
}

func reviewTask(id string) tasks.Task {
	return tasks.Task{ID: id, Title: id, Type: "review", TimeoutMinutes: 30, Prompt: "build it"}
}

// drain collects events until batch_end or timeout.
func drain(t *testing.T, r *Runner) []Event {
	t.Helper()
	ch, cancel := r.Subscribe()
	defer cancel()
	var events []Event
	deadline := time.After(15 * time.Second)
	for {
		select {
		case e := <-ch:
			events = append(events, e)
			if e.Type == "batch_end" {
				return events
			}
		case <-deadline:
			t.Fatalf("no batch_end; events so far: %+v", events)
		}
	}
}

// TestParseEventsRealStream pins the parser against an event stream captured
// from a real `opencode run --format json` invocation (2026-08, opencode 1.1.x):
// one assistant step answering "hello" with no tool calls.
func TestParseEventsRealStream(t *testing.T) {
	s := ParseEvents("testdata/real-events.jsonl")
	if s.Messages != 1 || s.ToolCalls != 0 {
		t.Fatalf("steps/tools: %+v", s)
	}
	if s.TokensIn != 7814 || s.TokensOut != 2 || s.TokensReasoning != 0 {
		t.Fatalf("tokens: %+v", s)
	}
	// text part window: 1786805672184 -> 1786805672297 = 113ms
	if s.GenSeconds < 0.112 || s.GenSeconds > 0.114 {
		t.Fatalf("gen seconds: %+v", s)
	}
}

func TestRunReviewTaskOK(t *testing.T) {
	r, st := newTestRunner(t, "ok")
	if err := r.StartBatch([]string{"model-a"}, []tasks.Task{reviewTask("tetris")}); err != nil {
		t.Fatal(err)
	}
	drain(t, r)
	m, _ := st.Latest()
	res := m["tetris"]["model-a"]
	if res.Status != "done" {
		t.Fatalf("status = %q (err %q), want done", res.Status, res.Error)
	}
	if res.Messages != 1 || res.ToolCalls != 1 || res.TokensIn != 100 || res.TokensOut != 200 {
		t.Fatalf("parsed stats wrong: %+v", res)
	}
	if res.TokensReasoning != 50 || res.CacheRead != 40 {
		t.Fatalf("reasoning/cache stats wrong: %+v", res)
	}
	if res.GenSeconds < 1.99 || res.GenSeconds > 2.01 {
		t.Fatalf("gen seconds = %v, want 2.0 from text part time span", res.GenSeconds)
	}
	ws := filepath.Join(st.RunPath(store.RunRef{Task: "tetris", Model: "model-a", Timestamp: res.Timestamp}), "workspace")
	if _, err := os.Stat(filepath.Join(ws, "hello.txt")); err != nil {
		t.Fatalf("workspace file missing: %v", err)
	}
}

func TestRunCheckTaskPassAndFail(t *testing.T) {
	r, st := newTestRunner(t, "ok")
	pass := tasks.Task{ID: "chk-pass", Title: "c", Type: "check", TimeoutMinutes: 30,
		Prompt: "p", Check: "test -f hello.txt"}
	fail := tasks.Task{ID: "chk-fail", Title: "c", Type: "check", TimeoutMinutes: 30,
		Prompt: "p", Check: "echo nope; false"}
	if err := r.StartBatch([]string{"m"}, []tasks.Task{pass, fail}); err != nil {
		t.Fatal(err)
	}
	drain(t, r)
	m, _ := st.Latest()
	if got := m["chk-pass"]["m"].Status; got != "pass" {
		t.Fatalf("pass task = %q", got)
	}
	res := m["chk-fail"]["m"]
	if res.Status != "fail" {
		t.Fatalf("fail task = %q", res.Status)
	}
	logPath := filepath.Join(st.RunPath(store.RunRef{Task: "chk-fail", Model: "m", Timestamp: res.Timestamp}), "check.log")
	raw, err := os.ReadFile(logPath)
	if err != nil || len(raw) == 0 {
		t.Fatalf("check.log missing: %v", err)
	}
}

func TestRunErrorMode(t *testing.T) {
	r, st := newTestRunner(t, "fail")
	r.StartBatch([]string{"m"}, []tasks.Task{reviewTask("tetris")})
	drain(t, r)
	m, _ := st.Latest()
	if got := m["tetris"]["m"].Status; got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
}

func TestRunTimeout(t *testing.T) {
	r, st := newTestRunner(t, "hang")
	r.Timeout = func(tasks.Task) time.Duration { return 500 * time.Millisecond }
	start := time.Now()
	r.StartBatch([]string{"m"}, []tasks.Task{reviewTask("tetris")})
	drain(t, r)
	if time.Since(start) > 10*time.Second {
		t.Fatal("timeout did not kill the process promptly")
	}
	m, _ := st.Latest()
	if got := m["tetris"]["m"].Status; got != "timeout" {
		t.Fatalf("status = %q, want timeout", got)
	}
}

func TestBusy(t *testing.T) {
	r, _ := newTestRunner(t, "hang")
	r.Timeout = func(tasks.Task) time.Duration { return 3 * time.Second }
	if err := r.StartBatch([]string{"m"}, []tasks.Task{reviewTask("tetris")}); err != nil {
		t.Fatal(err)
	}
	if err := r.StartBatch([]string{"m"}, []tasks.Task{reviewTask("tetris")}); err != ErrBusy {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	r.Cancel()
	drain(t, r)
}

func TestEventOrderModelMajor(t *testing.T) {
	r, _ := newTestRunner(t, "ok")
	ts := []tasks.Task{reviewTask("t1"), reviewTask("t2")}
	if err := r.StartBatch([]string{"m1", "m2"}, ts); err != nil {
		t.Fatal(err)
	}
	events := drain(t, r)
	var order []string
	for _, e := range events {
		if e.Type == "job_start" {
			order = append(order, e.Model+"/"+e.Task)
		}
	}
	want := []string{"m1/t1", "m1/t2", "m2/t1", "m2/t2"}
	if len(order) != 4 {
		t.Fatalf("job_starts = %v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}
