package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"opencode-bench/internal/store"
	"opencode-bench/internal/tasks"
)

func jsonUnmarshal(raw []byte, v any) error { return json.Unmarshal(raw, v) }

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
	// step window 40.387s; first-token wait (step_start 1786805631922 ->
	// text part start 1786805672184) = 40.262s of model swap + prefill,
	// leaving 0.125s of actual generation for the 2 output tokens.
	if s.LoadSeconds < 40.26 || s.LoadSeconds > 40.27 {
		t.Fatalf("load seconds: %+v", s)
	}
	if s.GenSeconds < 0.12 || s.GenSeconds > 0.13 {
		t.Fatalf("gen seconds: %+v", s)
	}
}

func TestRunReviewTaskOK(t *testing.T) {
	r, st := newTestRunner(t, "ok")
	if err := r.StartBatch(Pairs([]string{"model-a"}, []tasks.Task{reviewTask("tetris")})); err != nil {
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
	// step window 1.0s minus 0.4s first-token wait (step_start 1000 ->
	// first part start 1400)
	if res.GenSeconds < 0.59 || res.GenSeconds > 0.61 {
		t.Fatalf("gen seconds = %v, want 0.6", res.GenSeconds)
	}
	if res.LoadSeconds < 0.39 || res.LoadSeconds > 0.41 {
		t.Fatalf("load seconds = %v, want 0.4", res.LoadSeconds)
	}
	ws := filepath.Join(st.RunPath(store.RunRef{Task: "tetris", Model: "model-a", Timestamp: res.Timestamp}), "workspace")
	if _, err := os.Stat(filepath.Join(ws, "hello.txt")); err != nil {
		t.Fatalf("workspace file missing: %v", err)
	}
	// The workspace must be its own git repo so the agent's project-root
	// discovery anchors here instead of walking up into the harness repo.
	if _, err := os.Stat(filepath.Join(ws, ".git")); err != nil {
		t.Fatalf("workspace is not a git repo: %v", err)
	}
}

func TestProvenanceCaptured(t *testing.T) {
	r, st := newTestRunner(t, "ok")
	lsCfg := filepath.Join(t.TempDir(), "llama-swap.yaml")
	os.WriteFile(lsCfg, []byte(`
models:
  model-a:
    cmd: |
      llama-server --model /x/y.gguf --temp 0.7
    ttl: 300
  other:
    cmd: other-cmd
`), 0o644)
	r.LlamaSwapConfig = lsCfg

	task := reviewTask("tetris")
	if err := r.StartBatch(Pairs([]string{"model-a"}, []tasks.Task{task})); err != nil {
		t.Fatal(err)
	}
	drain(t, r)

	m, _ := st.Latest()
	res := m["tetris"]["model-a"]
	wantSHA := PromptSHA(task.Prompt)
	if res.PromptSHA != wantSHA {
		t.Fatalf("result prompt sha = %q, want %q", res.PromptSHA, wantSHA)
	}

	raw, err := os.ReadFile(filepath.Join(st.RunPath(store.RunRef{Task: "tetris", Model: "model-a", Timestamp: res.Timestamp}), "provenance.json"))
	if err != nil {
		t.Fatalf("provenance.json missing: %v", err)
	}
	var p map[string]any
	if err := jsonUnmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p["prompt_sha"] != wantSHA || p["model"] != "model-a" {
		t.Fatalf("provenance: %v", p)
	}
	entry, _ := p["llama_swap_entry"].(map[string]any)
	cmd, _ := entry["cmd"].(string)
	if !strings.Contains(cmd, "--temp 0.7") {
		t.Fatalf("llama-swap entry not captured: %v", entry)
	}
}

func TestProvenanceWithoutLlamaSwapConfig(t *testing.T) {
	r, st := newTestRunner(t, "ok")
	if err := r.StartBatch(Pairs([]string{"m"}, []tasks.Task{reviewTask("tetris")})); err != nil {
		t.Fatal(err)
	}
	drain(t, r)
	m, _ := st.Latest()
	if m["tetris"]["m"].PromptSHA == "" {
		t.Fatal("prompt sha should be recorded even without llama-swap config")
	}
}

func TestRunCheckTaskPassAndFail(t *testing.T) {
	r, st := newTestRunner(t, "ok")
	pass := tasks.Task{ID: "chk-pass", Title: "c", Type: "check", TimeoutMinutes: 30,
		Prompt: "p", Check: "test -f hello.txt"}
	fail := tasks.Task{ID: "chk-fail", Title: "c", Type: "check", TimeoutMinutes: 30,
		Prompt: "p", Check: "echo nope; false"}
	if err := r.StartBatch(Pairs([]string{"m"}, []tasks.Task{pass, fail})); err != nil {
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

func TestCheckPartialCreditRecorded(t *testing.T) {
	r, st := newTestRunner(t, "ok")
	task := tasks.Task{ID: "chk-partial", Title: "c", Type: "check", TimeoutMinutes: 30,
		Prompt: "p", Check: "echo 'FAIL agg-sum'; echo '3 assertion(s) failed'; exit 1"}
	r.StartBatch(Pairs([]string{"m"}, []tasks.Task{task}))
	drain(t, r)
	m, _ := st.Latest()
	res := m["chk-partial"]["m"]
	if res.Status != "fail" || !res.CheckParsed || res.CheckFailed != 3 {
		t.Fatalf("partial credit: %+v", res)
	}
}

func TestRunErrorMode(t *testing.T) {
	r, st := newTestRunner(t, "fail")
	r.StartBatch(Pairs([]string{"m"}, []tasks.Task{reviewTask("tetris")}))
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
	r.StartBatch(Pairs([]string{"m"}, []tasks.Task{reviewTask("tetris")}))
	drain(t, r)
	if time.Since(start) > 10*time.Second {
		t.Fatal("timeout did not kill the process promptly")
	}
	m, _ := st.Latest()
	if got := m["tetris"]["m"].Status; got != "timeout" {
		t.Fatalf("status = %q, want timeout", got)
	}
}

func TestCheckSurvivesPipeHoldingOrphan(t *testing.T) {
	r, st := newTestRunner(t, "ok")
	// The check backgrounds a long-lived child that inherits its fds, then
	// exits non-zero. Pipe-based capture would block until the child died;
	// file-based capture must return immediately.
	task := tasks.Task{ID: "chk-orphan", Title: "c", Type: "check", TimeoutMinutes: 30,
		Prompt: "p", Check: "echo starting; sleep 60 & echo backgrounded; exit 1"}
	start := time.Now()
	r.StartBatch(Pairs([]string{"m"}, []tasks.Task{task}))
	drain(t, r)
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("check with orphan took %s; runner wedged on inherited pipes", elapsed)
	}
	m, _ := st.Latest()
	res := m["chk-orphan"]["m"]
	if res.Status != "fail" {
		t.Fatalf("status = %q, want fail", res.Status)
	}
	logPath := filepath.Join(st.RunPath(store.RunRef{Task: "chk-orphan", Model: "m", Timestamp: res.Timestamp}), "check.log")
	raw, err := os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(raw), "backgrounded") {
		t.Fatalf("check.log content: %q err %v", raw, err)
	}
}

func TestCheckTimeoutKillsProcessGroup(t *testing.T) {
	r, st := newTestRunner(t, "ok")
	r.CheckTimeout = 500 * time.Millisecond
	task := tasks.Task{ID: "chk-hang", Title: "c", Type: "check", TimeoutMinutes: 30,
		Prompt: "p", Check: "echo begin; sleep 60; echo never"}
	start := time.Now()
	r.StartBatch(Pairs([]string{"m"}, []tasks.Task{task}))
	drain(t, r)
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("check timeout took %s", elapsed)
	}
	m, _ := st.Latest()
	res := m["chk-hang"]["m"]
	if res.Status != "fail" {
		t.Fatalf("status = %q, want fail", res.Status)
	}
	if !strings.Contains(res.Error, "check timeout") {
		t.Fatalf("error = %q, want check timeout mention", res.Error)
	}
}

func TestIdleTimeout(t *testing.T) {
	r, st := newTestRunner(t, "hang") // writes no events, sleeps 300s
	r.Timeout = func(tasks.Task) time.Duration { return 30 * time.Second }
	r.IdleTimeout = 400 * time.Millisecond
	start := time.Now()
	r.StartBatch(Pairs([]string{"m"}, []tasks.Task{reviewTask("tetris")}))
	drain(t, r)
	if time.Since(start) > 10*time.Second {
		t.Fatal("idle detection did not kill the silent job promptly")
	}
	m, _ := st.Latest()
	res := m["tetris"]["m"]
	if res.Status != "timeout" {
		t.Fatalf("status = %q, want timeout", res.Status)
	}
	if !strings.Contains(res.Error, "idle") {
		t.Fatalf("error = %q, want idle mention", res.Error)
	}
}

func TestIdleTimeoutDisabled(t *testing.T) {
	r, st := newTestRunner(t, "ok")
	r.IdleTimeout = 0 // disabled; normal fast run must be unaffected
	r.StartBatch(Pairs([]string{"m"}, []tasks.Task{reviewTask("tetris")}))
	drain(t, r)
	m, _ := st.Latest()
	if got := m["tetris"]["m"].Status; got != "done" {
		t.Fatalf("status = %q", got)
	}
}

func TestBusy(t *testing.T) {
	r, _ := newTestRunner(t, "hang")
	r.Timeout = func(tasks.Task) time.Duration { return 3 * time.Second }
	if err := r.StartBatch(Pairs([]string{"m"}, []tasks.Task{reviewTask("tetris")})); err != nil {
		t.Fatal(err)
	}
	if err := r.StartBatch(Pairs([]string{"m"}, []tasks.Task{reviewTask("tetris")})); err != ErrBusy {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	r.Cancel()
	drain(t, r)
}

func waitForJobStart(t *testing.T, r *Runner) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if running, cur, _, _ := r.Active(); running && cur.Type == "job_start" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no job started")
}

func TestCurrentRef(t *testing.T) {
	r, _ := newTestRunner(t, "hang")
	r.Timeout = func(tasks.Task) time.Duration { return 10 * time.Second }
	if _, ok := r.CurrentRef(); ok {
		t.Fatal("idle runner should have no current ref")
	}
	r.StartBatch(Pairs([]string{"m"}, []tasks.Task{reviewTask("t1")}))
	waitForJobStart(t, r)
	deadline := time.Now().Add(3 * time.Second)
	for {
		if ref, ok := r.CurrentRef(); ok {
			if ref.Task != "t1" || ref.Model != "m" || ref.Timestamp == "" {
				t.Fatalf("ref = %+v", ref)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no current ref while job running")
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.Cancel()
	drain(t, r)
	if _, ok := r.CurrentRef(); ok {
		t.Fatal("ref should clear after batch end")
	}
}

func TestTailEvents(t *testing.T) {
	tail := TailEvents("testdata/real-events.jsonl", 2)
	if len(tail) != 2 {
		t.Fatalf("tail = %d lines", len(tail))
	}
	var last map[string]any
	if err := json.Unmarshal(tail[1], &last); err != nil {
		t.Fatal(err)
	}
	if last["type"] != "step_finish" {
		t.Fatalf("last event = %v", last["type"])
	}
	if got := TailEvents("testdata/real-events.jsonl", 100); len(got) != 3 {
		t.Fatalf("over-sized tail = %d", len(got))
	}
	if got := TailEvents("testdata/does-not-exist.jsonl", 5); len(got) != 0 {
		t.Fatalf("missing file tail = %d", len(got))
	}
}

func TestBatchStateLifecycle(t *testing.T) {
	r, _ := newTestRunner(t, "ok")
	state := filepath.Join(t.TempDir(), "batch.json")
	r.StateFile = state
	if err := r.StartBatch(Pairs([]string{"m"}, []tasks.Task{reviewTask("t1"), reviewTask("t2")})); err != nil {
		t.Fatal(err)
	}
	drain(t, r)
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("state file should be removed after natural completion")
	}
}

func TestBatchStateOnUserCancel(t *testing.T) {
	r, _ := newTestRunner(t, "hang")
	r.Timeout = func(tasks.Task) time.Duration { return 10 * time.Second }
	state := filepath.Join(t.TempDir(), "batch.json")
	r.StateFile = state
	r.StartBatch(Pairs([]string{"m"}, []tasks.Task{reviewTask("t1"), reviewTask("t2")}))
	waitForJobStart(t, r)
	if _, err := os.Stat(state); err != nil {
		t.Fatalf("state file missing mid-batch: %v", err)
	}
	r.Cancel()
	drain(t, r)
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("state file should be removed on deliberate cancel")
	}
}

func TestBatchStateOnShutdown(t *testing.T) {
	r, _ := newTestRunner(t, "hang")
	r.Timeout = func(tasks.Task) time.Duration { return 10 * time.Second }
	state := filepath.Join(t.TempDir(), "batch.json")
	r.StateFile = state
	r.StartBatch(Pairs([]string{"m"}, []tasks.Task{reviewTask("t1"), reviewTask("t2")}))
	waitForJobStart(t, r)
	r.Shutdown()
	drain(t, r)
	raw, err := os.ReadFile(state)
	if err != nil {
		t.Fatalf("state file should survive shutdown: %v", err)
	}
	var st struct {
		Jobs []struct {
			Model string `json:"model"`
			Task  string `json:"task"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	// current (killed) job + the never-started one
	if len(st.Jobs) != 2 || st.Jobs[0].Task != "t1" || st.Jobs[1].Task != "t2" {
		t.Fatalf("pending jobs = %+v", st.Jobs)
	}
}

func TestEventOrderModelMajor(t *testing.T) {
	r, _ := newTestRunner(t, "ok")
	ts := []tasks.Task{reviewTask("t1"), reviewTask("t2")}
	if err := r.StartBatch(Pairs([]string{"m1", "m2"}, ts)); err != nil {
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
