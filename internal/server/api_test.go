package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"opencode-bench/internal/runner"
	"opencode-bench/internal/store"
	"opencode-bench/internal/tasks"
)

// newTestServer builds a fully wired server against temp dirs and the
// runner stub, returning the server plus its collaborators.
func newTestServer(t *testing.T, stubMode string) (*Server, *store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	occfg := filepath.Join(dir, "opencode.json")
	os.WriteFile(occfg, []byte(`{"provider":{"llama-swap":{"models":{
		"model-a":{"name":"Model A"},"model-b":{"name":"Model B"}}}}}`), 0o644)
	tasksDir := filepath.Join(dir, "tasks")
	os.MkdirAll(tasksDir, 0o755)
	os.WriteFile(filepath.Join(tasksDir, "tetris.yaml"), []byte(
		"id: tetris\ntitle: Tetris\ncategory: games\ntype: review\nprompt: build tetris\n"), 0o644)

	st := store.New(filepath.Join(dir, "runs"))
	stub, _ := filepath.Abs("../runner/testdata/opencode-stub.sh")
	os.Chmod(stub, 0o755)
	r := runner.New(stub, st)
	r.Timeout = func(tasks.Task) time.Duration { return 5 * time.Second }
	r.Env = append(os.Environ(), "STUB_MODE="+stubMode)
	stateFile := filepath.Join(dir, "runs", ".active-batch.json")
	r.StateFile = stateFile

	s := New(Config{
		OpencodeConfigPath: occfg,
		TasksDir:           tasksDir,
		RunsDir:            filepath.Join(dir, "runs"),
		BatchStateFile:     stateFile,
		Store:              st,
		Runner:             r,
	})
	return s, st, tasksDir
}

func doJSON(t *testing.T, s *Server, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	var out map[string]any
	json.Unmarshal(rr.Body.Bytes(), &out)
	return rr, out
}

func waitIdle(t *testing.T, s *Server) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if running, _, _, _ := s.cfg.Runner.Active(); !running {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("runner never went idle")
}

func TestGetModels(t *testing.T) {
	s, _, _ := newTestServer(t, "ok")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/models", nil))
	if rr.Code != 200 {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var models []map[string]string
	json.Unmarshal(rr.Body.Bytes(), &models)
	if len(models) != 2 || models[0]["id"] != "model-a" {
		t.Fatalf("models = %v", models)
	}
}

func TestTasksListDetailCreate(t *testing.T) {
	s, _, _ := newTestServer(t, "ok")
	_, body := doJSON(t, s, "GET", "/api/tasks", nil)
	if n := len(body["tasks"].([]any)); n != 1 {
		t.Fatalf("tasks = %d", n)
	}
	rr, detail := doJSON(t, s, "GET", "/api/tasks/tetris", nil)
	if rr.Code != 200 || detail["prompt"] != "build tetris" {
		t.Fatalf("detail: %d %v", rr.Code, detail)
	}
	if rr, _ := doJSON(t, s, "GET", "/api/tasks/nope", nil); rr.Code != 404 {
		t.Fatalf("missing task code = %d", rr.Code)
	}
	rr, _ = doJSON(t, s, "POST", "/api/tasks", map[string]any{
		"id": "kanban", "title": "Kanban", "category": "ui", "type": "review", "prompt": "build kanban"})
	if rr.Code != 201 {
		t.Fatalf("create code = %d", rr.Code)
	}
	rr, _ = doJSON(t, s, "POST", "/api/tasks", map[string]any{
		"id": "bad", "title": "Bad", "type": "check", "prompt": "p"}) // no check script
	if rr.Code != 400 {
		t.Fatalf("invalid create code = %d", rr.Code)
	}
}

func TestRunsLifecycle(t *testing.T) {
	s, _, _ := newTestServer(t, "ok")
	rr, body := doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}})
	if rr.Code != 202 || body["jobs"].(float64) != 1 {
		t.Fatalf("start: %d %v", rr.Code, body)
	}
	waitIdle(t, s)

	_, matrix := doJSON(t, s, "GET", "/api/runs", nil)
	cell := matrix["matrix"].(map[string]any)["tetris"].(map[string]any)["model-a"].(map[string]any)
	if cell["status"] != "done" {
		t.Fatalf("cell = %v", cell)
	}

	rr, _ = doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"nope"}})
	if rr.Code != 400 {
		t.Fatalf("unknown task code = %d", rr.Code)
	}
	rr, _ = doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"ghost"}, "tasks": []string{"tetris"}})
	if rr.Code != 400 {
		t.Fatalf("unknown model code = %d", rr.Code)
	}

	// history + detail + files + preview
	req := httptest.NewRequest("GET", "/api/runs/tetris/model-a", nil)
	rr2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr2, req)
	var hist []map[string]any
	json.Unmarshal(rr2.Body.Bytes(), &hist)
	if len(hist) != 1 {
		t.Fatalf("history = %v", hist)
	}
	ts := hist[0]["timestamp"].(string)

	_, detail := doJSON(t, s, "GET", "/api/runs/tetris/model-a/"+ts, nil)
	if detail["result"].(map[string]any)["status"] != "done" {
		t.Fatalf("detail = %v", detail)
	}
	if n := len(detail["events"].([]any)); n != 4 {
		t.Fatalf("events = %d", n)
	}

	rr3 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr3, httptest.NewRequest("GET", "/api/runs/tetris/model-a/"+ts+"/files", nil))
	var files []map[string]any
	json.Unmarshal(rr3.Body.Bytes(), &files)
	if len(files) != 1 || files[0]["path"] != "hello.txt" {
		t.Fatalf("files = %v", files)
	}

	rr4 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr4, httptest.NewRequest("GET", "/api/runs/tetris/model-a/"+ts+"/preview/hello.txt", nil))
	if rr4.Code != 200 || rr4.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("preview: %d csp=%q", rr4.Code, rr4.Header().Get("Content-Security-Policy"))
	}

	// path traversal
	rr5 := httptest.NewRecorder()
	req5 := httptest.NewRequest("GET", "/api/runs/tetris/model-a/"+ts+"/files/../../../etc/passwd", nil)
	req5.URL.Path = "/api/runs/tetris/model-a/" + ts + "/files/../../../etc/passwd" // bypass client normalization
	s.Handler().ServeHTTP(rr5, req5)
	if rr5.Code == 200 && strings.Contains(rr5.Body.String(), "root:") {
		t.Fatalf("path traversal served /etc/passwd")
	}
}

func TestSamplesEnsureCount(t *testing.T) {
	s, _, _ := newTestServer(t, "ok")
	// fresh cell, samples=3 -> 3 jobs
	rr, resp := doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}, "samples": 3})
	if rr.Code != 202 || resp["jobs"].(float64) != 3 {
		t.Fatalf("fresh samples=3: %d %v", rr.Code, resp)
	}
	waitIdle(t, s)
	// now 3 completed; samples=3 -> nothing; samples=5 -> 2 more
	rr, resp = doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}, "samples": 3})
	if rr.Code != 200 || resp["jobs"].(float64) != 0 {
		t.Fatalf("satisfied samples=3: %d %v", rr.Code, resp)
	}
	rr, resp = doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}, "samples": 5})
	if rr.Code != 202 || resp["jobs"].(float64) != 2 {
		t.Fatalf("top-up to 5: %d %v", rr.Code, resp)
	}
	waitIdle(t, s)
	// force + samples=2 -> exactly 2 more regardless of history
	rr, resp = doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}, "samples": 2, "force": true})
	if rr.Code != 202 || resp["jobs"].(float64) != 2 {
		t.Fatalf("force samples=2: %d %v", rr.Code, resp)
	}
	waitIdle(t, s)

	// matrix aggregation reflects all 7 samples
	_, matrix := doJSON(t, s, "GET", "/api/runs", nil)
	agg := matrix["agg"].(map[string]any)["tetris"].(map[string]any)["model-a"].(map[string]any)
	if agg["samples"].(float64) != 7 || agg["dones"].(float64) != 7 {
		t.Fatalf("agg: %v", agg)
	}
}

func TestResumableBatchLifecycle(t *testing.T) {
	s, _, _ := newTestServer(t, "ok")
	// no state file yet
	rr, _ := doJSON(t, s, "GET", "/api/runs/resumable", nil)
	if rr.Code != 404 {
		t.Fatalf("resumable with no state = %d", rr.Code)
	}
	// simulate an interrupted batch
	os.MkdirAll(filepath.Dir(s.cfg.BatchStateFile), 0o755)
	os.WriteFile(s.cfg.BatchStateFile, []byte(
		`{"jobs":[{"model":"model-a","task":"tetris"},{"model":"model-a","task":"ghost-task"}]}`), 0o644)

	rr, body := doJSON(t, s, "GET", "/api/runs/resumable", nil)
	if rr.Code != 200 || body["count"].(float64) != 2 {
		t.Fatalf("resumable = %d %v", rr.Code, body)
	}

	rr, body = doJSON(t, s, "POST", "/api/runs/resume", nil)
	if rr.Code != 202 || body["jobs"].(float64) != 1 || body["dropped"].(float64) != 1 {
		t.Fatalf("resume = %d %v (ghost task should be dropped)", rr.Code, body)
	}
	waitIdle(t, s)
	rr, _ = doJSON(t, s, "GET", "/api/runs/resumable", nil)
	if rr.Code != 404 {
		t.Fatalf("state should be gone after resumed batch completes: %d", rr.Code)
	}
}

func TestResumableDismiss(t *testing.T) {
	s, _, _ := newTestServer(t, "ok")
	os.MkdirAll(filepath.Dir(s.cfg.BatchStateFile), 0o755)
	os.WriteFile(s.cfg.BatchStateFile, []byte(`{"jobs":[{"model":"model-a","task":"tetris"}]}`), 0o644)
	rr, _ := doJSON(t, s, "DELETE", "/api/runs/resumable", nil)
	if rr.Code != 204 {
		t.Fatalf("dismiss = %d", rr.Code)
	}
	rr, _ = doJSON(t, s, "GET", "/api/runs/resumable", nil)
	if rr.Code != 404 {
		t.Fatalf("state survived dismiss: %d", rr.Code)
	}
}

func TestSkipCompletedByDefault(t *testing.T) {
	s, _, _ := newTestServer(t, "ok")
	body := map[string]any{"models": []string{"model-a"}, "tasks": []string{"tetris"}}
	rr, resp := doJSON(t, s, "POST", "/api/runs", body)
	if rr.Code != 202 || resp["jobs"].(float64) != 1 {
		t.Fatalf("first run: %d %v", rr.Code, resp)
	}
	waitIdle(t, s)

	rr, resp = doJSON(t, s, "POST", "/api/runs", body)
	if rr.Code != 200 || resp["jobs"].(float64) != 0 || resp["skipped"].(float64) != 1 {
		t.Fatalf("rerun should skip completed: %d %v", rr.Code, resp)
	}

	rr, resp = doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}, "force": true})
	if rr.Code != 202 || resp["jobs"].(float64) != 1 || resp["skipped"].(float64) != 0 {
		t.Fatalf("force should re-run: %d %v", rr.Code, resp)
	}
	waitIdle(t, s)
}

func TestSkipLooksAtFullHistoryNotJustLatest(t *testing.T) {
	s, st, _ := newTestServer(t, "ok")
	// older completed run, newer cancelled/errored run in the same cell
	ref1, _, _ := st.NewRunDir("tetris", "model-a")
	st.WriteResult(ref1, store.Result{Task: "tetris", Model: "model-a", Status: "done", Timestamp: ref1.Timestamp})
	time.Sleep(1100 * time.Millisecond)
	ref2, _, _ := st.NewRunDir("tetris", "model-a")
	st.WriteResult(ref2, store.Result{Task: "tetris", Model: "model-a", Status: "error", Error: "cancelled", Timestamp: ref2.Timestamp})

	rr, resp := doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}})
	if rr.Code != 200 || resp["jobs"].(float64) != 0 || resp["skipped"].(float64) != 1 {
		t.Fatalf("cell with a historical success should be skipped: %d %v", rr.Code, resp)
	}
}

func TestErroredRunsRetryByDefault(t *testing.T) {
	s, _, _ := newTestServer(t, "fail")
	body := map[string]any{"models": []string{"model-a"}, "tasks": []string{"tetris"}}
	rr, _ := doJSON(t, s, "POST", "/api/runs", body)
	if rr.Code != 202 {
		t.Fatalf("first run: %d", rr.Code)
	}
	waitIdle(t, s)

	rr, resp := doJSON(t, s, "POST", "/api/runs", body)
	if rr.Code != 202 || resp["jobs"].(float64) != 1 {
		t.Fatalf("errored cell should retry without force: %d %v", rr.Code, resp)
	}
	waitIdle(t, s)
}

func TestStaleDetectionAndProvenance(t *testing.T) {
	s, _, tasksDir := newTestServer(t, "ok")
	rr, _ := doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}})
	if rr.Code != 202 {
		t.Fatalf("start = %d", rr.Code)
	}
	waitIdle(t, s)

	_, matrix := doJSON(t, s, "GET", "/api/runs", nil)
	cell := matrix["matrix"].(map[string]any)["tetris"].(map[string]any)["model-a"].(map[string]any)
	if cell["stale"] == true {
		t.Fatal("fresh run marked stale")
	}
	if cell["prompt_sha"] == nil || cell["prompt_sha"] == "" {
		t.Fatal("prompt_sha missing from cell")
	}
	ts := cell["timestamp"].(string)

	_, detail := doJSON(t, s, "GET", "/api/runs/tetris/model-a/"+ts, nil)
	prov, _ := detail["provenance"].(map[string]any)
	if prov == nil || prov["prompt_sha"] != cell["prompt_sha"] {
		t.Fatalf("provenance missing from detail: %v", detail["provenance"])
	}

	// change the prompt -> run becomes stale
	os.WriteFile(filepath.Join(tasksDir, "tetris.yaml"), []byte(
		"id: tetris\ntitle: Tetris\ncategory: games\ntype: review\nprompt: build BETTER tetris\n"), 0o644)
	_, matrix = doJSON(t, s, "GET", "/api/runs", nil)
	cell = matrix["matrix"].(map[string]any)["tetris"].(map[string]any)["model-a"].(map[string]any)
	if cell["stale"] != true {
		t.Fatalf("run not marked stale after prompt change: %v", cell)
	}
}

func TestVerdictLifecycle(t *testing.T) {
	s, _, _ := newTestServer(t, "ok")
	rr, _ := doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}})
	if rr.Code != 202 {
		t.Fatalf("start = %d", rr.Code)
	}
	waitIdle(t, s)
	_, matrix := doJSON(t, s, "GET", "/api/runs", nil)
	ts := matrix["matrix"].(map[string]any)["tetris"].(map[string]any)["model-a"].(map[string]any)["timestamp"].(string)
	base := "/api/runs/tetris/model-a/" + ts

	rr, _ = doJSON(t, s, "POST", base+"/verdict", map[string]any{"verdict": "good", "note": "works"})
	if rr.Code != 200 {
		t.Fatalf("set verdict = %d %s", rr.Code, rr.Body.String())
	}
	_, detail := doJSON(t, s, "GET", base, nil)
	v := detail["result"].(map[string]any)["verdict"]
	if v == nil || v.(map[string]any)["verdict"] != "good" {
		t.Fatalf("verdict not in detail: %v", v)
	}
	_, matrix = doJSON(t, s, "GET", "/api/runs", nil)
	cell := matrix["matrix"].(map[string]any)["tetris"].(map[string]any)["model-a"].(map[string]any)
	if cell["verdict"] == nil {
		t.Fatalf("verdict not in matrix cell: %v", cell)
	}

	rr, _ = doJSON(t, s, "POST", base+"/verdict", map[string]any{"verdict": "meh"})
	if rr.Code != 400 {
		t.Fatalf("invalid verdict = %d", rr.Code)
	}
	rr, _ = doJSON(t, s, "POST", base+"/verdict", map[string]any{"rating": 7, "note": "mostly works"})
	if rr.Code != 200 {
		t.Fatalf("rating verdict = %d %s", rr.Code, rr.Body.String())
	}
	_, detail = doJSON(t, s, "GET", base, nil)
	if v := detail["result"].(map[string]any)["verdict"]; v == nil || v.(map[string]any)["rating"].(float64) != 7 {
		t.Fatalf("rating not in detail: %v", v)
	}
	rr, _ = doJSON(t, s, "POST", base+"/verdict", map[string]any{"rating": 12})
	if rr.Code != 400 {
		t.Fatalf("out-of-range rating = %d", rr.Code)
	}
	rr, _ = doJSON(t, s, "POST", "/api/runs/tetris/model-a/2020-01-01T00-00-00Z/verdict", map[string]any{"verdict": "good"})
	if rr.Code != 404 {
		t.Fatalf("verdict on missing run = %d", rr.Code)
	}
	rr, _ = doJSON(t, s, "DELETE", base+"/verdict", nil)
	if rr.Code != 204 {
		t.Fatalf("clear = %d", rr.Code)
	}
	_, detail = doJSON(t, s, "GET", base, nil)
	if detail["result"].(map[string]any)["verdict"] != nil {
		t.Fatal("verdict survived clear")
	}
}

func TestReviewsPending(t *testing.T) {
	s, st, _ := newTestServer(t, "ok")
	write := func(task, model, status, verdict string) {
		ref, _, _ := st.NewRunDir(task, model)
		st.WriteResult(ref, store.Result{Task: task, Model: model, Status: status, Timestamp: ref.Timestamp})
		if verdict != "" {
			st.WriteVerdict(ref, store.Verdict{Verdict: verdict})
		}
	}
	write("tetris", "model-a", "done", "")     // pending
	write("tetris", "model-b", "done", "good") // judged
	write("checkers", "model-a", "done", "")   // pending
	write("chk", "model-a", "pass", "")        // check run: never pending
	write("kanban", "model-a", "error", "")    // not a completed review

	rr, _ := doJSON(t, s, "GET", "/api/reviews/pending", nil)
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	var pending []map[string]string
	json.Unmarshal(rr.Body.Bytes(), &pending)
	if len(pending) != 2 {
		t.Fatalf("pending = %v", pending)
	}
	seen := map[string]bool{}
	for _, p := range pending {
		if p["task"] == "" || p["model"] == "" || p["timestamp"] == "" {
			t.Fatalf("incomplete entry: %v", p)
		}
		seen[p["task"]] = true
	}
	if !seen["tetris"] || !seen["checkers"] {
		t.Fatalf("wrong tasks: %v", pending)
	}
}

func TestLeaderboard(t *testing.T) {
	s, st, _ := newTestServer(t, "ok")
	write := func(task, model, status string, tps float64, verdict string) {
		ref, _, _ := st.NewRunDir(task, model)
		r := store.Result{Task: task, Model: model, Status: status, Timestamp: ref.Timestamp,
			TokensOut: int(tps * 10), GenSeconds: 10}
		st.WriteResult(ref, r)
		if verdict != "" {
			st.WriteVerdict(ref, store.Verdict{Verdict: verdict})
		}
	}
	writeRated := func(task, model string, rating int) {
		ref, _, _ := st.NewRunDir(task, model)
		st.WriteResult(ref, store.Result{Task: task, Model: model, Status: "done", Timestamp: ref.Timestamp})
		st.WriteVerdict(ref, store.Verdict{Rating: rating})
	}
	// model-a: 2 check cells (1 pass, 1 fail), 1 review judged good, 2 rated
	write("chk1", "model-a", "pass", 50, "")
	write("chk2", "model-a", "fail", 70, "")
	write("rev1", "model-a", "done", 60, "good")
	writeRated("rev2", "model-a", 9)
	writeRated("rev3", "model-a", 5)
	// model-b: 1 check pass, 1 review judged bad, 1 error
	write("chk1", "model-b", "pass", 100, "")
	write("rev1", "model-b", "done", 90, "bad")
	write("chk2", "model-b", "error", 0, "")

	rr, _ := doJSON(t, s, "GET", "/api/leaderboard", nil)
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	var rows []map[string]any
	json.Unmarshal(rr.Body.Bytes(), &rows)
	byModel := map[string]map[string]any{}
	for _, r := range rows {
		byModel[r["model"].(string)] = r
	}
	a := byModel["model-a"]
	if a["check_cells_passed"].(float64) != 1 || a["check_cells"].(float64) != 2 {
		t.Fatalf("model-a checks: %v", a)
	}
	if a["verdict_good"].(float64) != 1 || a["reviews_done"].(float64) != 3 {
		t.Fatalf("model-a reviews: %v", a)
	}
	if a["rating_count"].(float64) != 2 || a["rating_avg"].(float64) != 7 {
		t.Fatalf("model-a ratings: %v", a)
	}
	if tps := a["median_tps"].(float64); tps < 59 || tps > 61 {
		t.Fatalf("model-a tps: %v", tps)
	}
	b := byModel["model-b"]
	if b["errors"].(float64) != 1 || b["verdict_bad"].(float64) != 1 {
		t.Fatalf("model-b: %v", b)
	}
}

func TestActiveTail(t *testing.T) {
	s, _, _ := newTestServer(t, "hang")
	rr, _ := doJSON(t, s, "GET", "/api/runs/active/tail", nil)
	if rr.Code != 404 {
		t.Fatalf("idle tail = %d", rr.Code)
	}
	rr, _ = doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}})
	if rr.Code != 202 {
		t.Fatalf("start = %d", rr.Code)
	}
	deadline := time.Now().Add(3 * time.Second)
	var body map[string]any
	for {
		rr, body = doJSON(t, s, "GET", "/api/runs/active/tail", nil)
		if rr.Code == 200 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("tail never became available: %d", rr.Code)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if body["task"] != "tetris" || body["model"] != "model-a" || body["timestamp"] == "" {
		t.Fatalf("tail identity: %v", body)
	}
	// hang stub writes no events: empty tail, age -1
	if n := len(body["recent"].([]any)); n != 0 {
		t.Fatalf("recent = %d events for silent job", n)
	}
	if body["last_event_age_sec"].(float64) != -1 {
		t.Fatalf("age = %v, want -1 with no events", body["last_event_age_sec"])
	}
	s.cfg.Runner.Cancel()
	waitIdle(t, s)
}

func TestRunsBusy409(t *testing.T) {
	s, _, _ := newTestServer(t, "hang")
	rr, _ := doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}})
	if rr.Code != 202 {
		t.Fatalf("start = %d", rr.Code)
	}
	rr, _ = doJSON(t, s, "POST", "/api/runs", map[string]any{
		"models": []string{"model-a"}, "tasks": []string{"tetris"}})
	if rr.Code != 409 {
		t.Fatalf("busy = %d", rr.Code)
	}
	rr, _ = doJSON(t, s, "DELETE", "/api/runs/active", nil)
	if rr.Code != 204 {
		t.Fatalf("cancel = %d", rr.Code)
	}
	waitIdle(t, s)
}
