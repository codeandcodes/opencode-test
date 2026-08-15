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

	s := New(Config{
		OpencodeConfigPath: occfg,
		TasksDir:           tasksDir,
		RunsDir:            filepath.Join(dir, "runs"),
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
