# opencode-bench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A single-binary Go + Svelte 5 web app that runs complex coding tasks against every local model in opencode's `llama-swap` provider and captures traces, outputs, and pass/fail results for comparison.

**Architecture:** Go backend shells out to `opencode run --format json --auto` in per-run scratch workspaces, serially, grouped by model; results are plain files under `runs/`; a Svelte 5 UI (embedded via go:embed) shows a task×model matrix, run details with iframe preview, and side-by-side compare.

**Tech Stack:** Go 1.26+, gopkg.in/yaml.v3, net/http + ServeMux patterns, SSE; Svelte 5 (runes) + Vite + TypeScript; vitest. No database.

**Spec:** `docs/superpowers/specs/2026-08-15-opencode-bench-design.md`

## Global Constraints

- Module name: `opencode-bench`; binary listens on `127.0.0.1:7777` by default (`-listen` flag).
- Model source of truth: `~/.config/opencode/opencode.json` → `provider["llama-swap"].models` (path overridable with `-opencode-config`); read per-request, never cached.
- Runs layout (exact): `runs/<taskID>/<modelID>/<RFC3339 timestamp>/` containing `workspace/`, `events.jsonl`, `stderr.log`, optional `check.log`, `result.json`.
- Statuses (exact strings): `done`, `pass`, `fail`, `timeout`, `error`, `interrupted`, `running`.
- One active batch; second `POST /api/runs` → HTTP 409. Jobs ordered model-major (all tasks for model A, then model B).
- opencode invocation (exact): `opencode run --dir <ws> -m llama-swap/<model> --format json --auto <prompt>`; process started with Setpgid, killed by process group on timeout/cancel.
- Check scripts run `bash -c <script>` with cwd = workspace, 5-minute timeout.
- All Go tests must run without GPU, network, or real opencode (stub script pattern).
- Review-task prompts MUST demand a self-contained static web app: `index.html` entry, no build step, no external network. This sentence appears verbatim as a constraint block in every review task prompt.
- Commit after every green test cycle; trailers per session rules.

---

### Task 1: Scaffold + healthz

**Files:**

- Create: `go.mod`, `main.go`, `Makefile`, `.gitignore`, `internal/server/server.go`, `internal/server/server_test.go`

**Interfaces:**

- Produces: `server.New(cfg server.Config) *server.Server` with `func (s *Server) Handler() http.Handler`; `server.Config{OpencodeConfigPath, TasksDir, RunsDir, OpencodeBin string}`. `GET /healthz` → 200 `{"ok":true}`.

- [ ] **Step 1: Failing test** — `internal/server/server_test.go`:

```go
package server

import (
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	s := New(Config{})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/healthz", nil))
	if rr.Code != 200 {
		t.Fatalf("got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/server/` — expect FAIL (undefined: New).
- [ ] **Step 3: Implement** minimal `server.go` (Config struct, Server struct holding mux, `New` registers `/healthz` returning `{"ok":true}` with `application/json`).
- [ ] **Step 4:** `go test ./...` — PASS. `main.go`: flags `-listen`, `-opencode-config` (default `$HOME/.config/opencode/opencode.json`), `-tasks` (default `tasks`), `-runs` (default `runs`), `-opencode` (default `opencode`); `http.ListenAndServe`. `.gitignore`: `runs/`, `ui/node_modules/`, `ui/dist/`, `opencode-bench`. Makefile targets: `build`, `test`, `run`, `ui`, `check` (test + vet).
- [ ] **Step 5: Commit** `feat: scaffold opencode-bench server with healthz`

### Task 2: Model discovery (`internal/config`)

**Files:**

- Create: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**

- Produces: `config.Model{ID, Name string}`; `config.DiscoverModels(path string) ([]Model, error)` — sorted by ID; error if file missing/unparseable or provider absent.

- [ ] **Step 1: Failing tests** — table: (a) valid file with 2 models incl. `name` field → 2 sorted Models; (b) missing `llama-swap` provider → error containing `llama-swap`; (c) nonexistent path → error. Fixture JSON written via `t.TempDir()`:

```go
const sample = `{"provider":{"llama-swap":{"models":{
 "b-model":{"name":"B"},"a-model":{"name":"A"}}}}}`
```

- [ ] **Step 2:** Run — FAIL. **Step 3:** Implement with `encoding/json` into `map[string]struct{ Models map[string]struct{ Name string } }` shaped struct. **Step 4:** PASS.
- [ ] **Step 5: Commit** `feat: discover models from opencode llama-swap provider`

### Task 3: Task library (`internal/tasks`)

**Files:**

- Create: `internal/tasks/tasks.go`, `internal/tasks/tasks_test.go`

**Interfaces:**

- Produces: `tasks.Task{ID, Title, Category, Type string; TimeoutMinutes int; Prompt, Check string}`; `tasks.Library{Tasks []Task; Warnings []string}`; `tasks.Load(dir string) (Library, error)`; `tasks.Save(dir string, t Task) error`.
- Validation: ID must match filename stem and `^[a-z0-9-]+$`; Type ∈ {review, check}; Prompt non-empty; Check non-empty iff Type==check; TimeoutMinutes default 30. Invalid file → entry in Warnings (`"<file>: <reason>"`), not fatal. Duplicate ID → warning, first wins. Tasks sorted: reviews first, then checks, each alphabetical.

- [ ] **Step 1: Failing tests**: valid review + check YAML load; check-without-script → warning; bad type → warning; default timeout applied; Save round-trips through Load and rejects invalid Task with error.
- [ ] **Step 2:** FAIL. **Step 3:** Implement with `gopkg.in/yaml.v3` (`go get gopkg.in/yaml.v3`). **Step 4:** PASS.
- [ ] **Step 5: Commit** `feat: task library loader with validation and warnings`

### Task 4: Store (`internal/store`)

**Files:**

- Create: `internal/store/store.go`, `internal/store/result.go`, `internal/store/store_test.go`

**Interfaces:**

- Produces:
  - `store.Result{Task, Model, Status string; StartedAt, FinishedAt time.Time; DurationSec float64; Messages, ToolCalls, TokensIn, TokensOut int; Error string; Timestamp string}` (Timestamp = run dir name).
  - `store.New(root string) *Store`
  - `(*Store) NewRunDir(task, model string) (ref RunRef, workspace string, err error)` — `RunRef{Task, Model, Timestamp string}`; timestamp `time.Now().UTC().Format("2006-01-02T15-04-05Z")` (colon-free RFC3339 variant, filesystem-safe).
  - `(*Store) WriteResult(ref RunRef, r Result) error` / `(*Store) ReadResult(ref RunRef) (Result, error)`
  - `(*Store) Latest() (map[string]map[string]Result, error)` — latest per (task, model); a run dir lacking `result.json` yields `Status:"interrupted"`.
  - `(*Store) History(task, model string) ([]Result, error)` — newest first.
  - `(*Store) RunPath(ref RunRef) string`; `(*Store) SafePath(ref RunRef, rel string) (string, error)` — reject any resolved path escaping the run dir (use `filepath.Rel` check after `filepath.Clean`; reject `..` components); `(*Store) ListFiles(ref RunRef) ([]FileEntry, error)` with `FileEntry{Path string; Size int64; Dir bool}` (workspace-relative, sorted).

- [ ] **Step 1: Failing tests**: NewRunDir creates dirs; Write/Read round-trip; Latest picks newest and marks result-less dir interrupted; History ordering; SafePath rejects `../escape` and absolute paths, accepts `sub/file.txt`; ListFiles walks workspace only.
- [ ] **Step 2:** FAIL. **Step 3:** Implement (pure stdlib). **Step 4:** PASS.
- [ ] **Step 5: Commit** `feat: filesystem run store with path-safe file access`

### Task 5: Events parsing + Runner (`internal/runner`)

**Files:**

- Create: `internal/runner/events.go`, `internal/runner/runner.go`, `internal/runner/runner_test.go`, `internal/runner/testdata/opencode-stub.sh`

**Interfaces:**

- Consumes: `store.Store`, `tasks.Task`.
- Produces:
  - `runner.ParseEvents(path string) (messages, toolCalls, tokensIn, tokensOut int)` — tolerant line-scan: each line JSON→`map[string]any`; `type` containing `"message"` increments messages; `type` containing `"tool"` increments toolCalls; any object at key `tokens`/`usage` with numeric `input`/`output` adds to token counts. Unparseable lines skipped.
  - `runner.Event{Type string; Task, Model, Status string; Done, Total int}` with Type ∈ {`job_start`,`job_end`,`batch_end`}.
  - `runner.New(opencodeBin string, st *store.Store) *Runner`
  - `(*Runner) StartBatch(models []string, ts []tasks.Task) error` — `ErrBusy` if active; jobs = model-major order; runs in one goroutine.
  - `(*Runner) Cancel()`; `(*Runner) Active() (running bool, cur Event, done, total int)`
  - `(*Runner) Subscribe() (ch <-chan Event, unsubscribe func())` — non-blocking fan-out (drop on slow consumer).
  - Job execution: per Global Constraints; timeout `task.TimeoutMinutes`; on exec success + type=check → run check via `bash -c` in workspace, 5 min timeout, output→`check.log`, exit0→pass else fail; type=review → `done`; non-zero opencode exit or empty events.jsonl → `error`; deadline → `timeout`; cancelled current job → `error` with Error `"cancelled"`, remaining jobs skipped.

- [ ] **Step 1: Stub** `testdata/opencode-stub.sh` (chmod +x): reads args; behavior keyed on env `STUB_MODE`: `ok` → emit 3 JSON event lines (message, tool, message-with-tokens) to stdout and create `hello.txt` in `--dir`; `fail` → exit 3; `hang` → sleep 300. Test passes stub path as opencodeBin and small TimeoutMinutes via a test-only seam: TimeoutMinutes is on Task, so use a task with `TimeoutMinutes: 0` interpreted as "1 second" ONLY when env `BENCH_TEST_FAST=1`? — NO. Simpler, no seams: change TimeoutMinutes semantics to float minutes is ugly; instead Runner takes `Timeout func(tasks.Task) time.Duration` field defaulting to minutes — tests override to 500ms. Lock this in: `Runner.Timeout` is exported.
- [ ] **Step 2: Failing tests**: (a) ok-mode review task → result done, events file has 3 lines, ParseEvents counts (2 messages, 1 tool, tokens); (b) ok-mode check task with check `test -f hello.txt` → pass; check `false` → fail with check.log; (c) fail mode → error; (d) hang mode with 500ms timeout → timeout, and process actually dead (poll /proc or `Wait` returns); (e) StartBatch while active → ErrBusy; (f) events received in order job_start/job_end×N/batch_end; (g) model-major ordering asserted from event sequence.
- [ ] **Step 3:** FAIL → implement → PASS. Use `exec.CommandContext` + `SysProcAttr{Setpgid: true}` + `syscall.Kill(-pid, SIGKILL)` on context done.
- [ ] **Step 4: Commit** `feat: serial batch runner with stubbed opencode tests`

### Task 6: Full HTTP API (`internal/server`)

**Files:**

- Modify: `internal/server/server.go`; Create: `internal/server/api.go`, `internal/server/sse.go`, `internal/server/api_test.go`

**Interfaces:**

- Consumes: everything above. Server Config gains `Runner *runner.Runner`, `Store *store.Store`.
- Produces routes (all JSON unless noted):
  - `GET /api/models` → `[{"id","name"}]` (503 + error JSON if config unreadable)
  - `GET /api/tasks` → `{"tasks":[{id,title,category,type,timeout_minutes}],"warnings":[...]}`
  - `GET /api/tasks/{id}` → full task incl. prompt/check; 404 unknown
  - `POST /api/tasks` → body = full task JSON; `tasks.Save`; 400 on validation error
  - `POST /api/runs` body `{"models":["m"],"tasks":["t"]}` → 202 `{"jobs":N}`; 409 if busy; 400 on unknown model/task id
  - `DELETE /api/runs/active` → 204
  - `GET /api/runs` → `{"matrix":{task:{model:Result}},"active":{running,task,model,done,total}}`
  - `GET /api/runs/{task}/{model}` → `[Result]` history
  - `GET /api/runs/{task}/{model}/{ts}` → `{"result":Result,"events":[<raw JSON lines>]}` (events capped at 2000 lines)
  - `GET /api/runs/{task}/{model}/{ts}/files` → `[FileEntry]`
  - `GET /api/runs/{task}/{model}/{ts}/files/{path...}` → raw bytes, `Content-Type` by extension
  - `GET /api/runs/{task}/{model}/{ts}/preview/{path...}` → same bytes + header `Content-Security-Policy: default-src 'self' 'unsafe-inline' 'unsafe-eval' data: blob:` (allows the app's own files, blocks external network)
  - `GET /api/events` → SSE (`text/event-stream`), forwards runner events as `data: {json}\n\n`, plus `: ping` comment every 15s
- [ ] **Step 1: Failing httptest tests** covering: models happy path (temp config file), tasks list+detail+create+invalid, run start 202/409/400 (use stub runner via real Runner with stub bin), matrix shape after a completed run, history, detail events cap, files + preview headers, path traversal 400/404 (`files/../../etc/passwd`).
- [ ] **Step 2:** FAIL → implement (Go 1.22 mux patterns `GET /api/runs/{task}/{model}/{ts}/files/{path...}`) → PASS.
- [ ] **Step 3: Commit** `feat: complete REST+SSE API`

### Task 7: UI scaffold, api client, Matrix + RunControl

**Files:**

- Create: `ui/` via `npm create vite@latest ui -- --template svelte-ts` (Svelte 5), `ui/src/lib/api.ts`, `ui/src/lib/sse.ts`, `ui/src/lib/types.ts`, `ui/src/App.svelte`, `ui/src/lib/Matrix.svelte`, `ui/src/lib/RunControl.svelte`, `ui/vite.config.ts` (proxy `/api` → `http://127.0.0.1:7777`)

**Interfaces:**

- Produces: `api.ts` typed fetch helpers (`getModels`, `getTasks`, `getMatrix`, `startRuns`, `cancelRuns`, `getHistory`, `getRun`, `getFiles`, `createTask`); `sse.ts` `subscribe(onEvent)` with auto-reconnect; hash router in `App.svelte`: `#/` matrix, `#/run/{task}/{model}/{ts}` detail, `#/compare` compare, `#/new` new-task.
- Matrix: rows grouped by category, review/check badge per row; columns = models; cell shows status chip (colors: pass=green, fail=red, done=blue "review", timeout/orange, error/red-outline, interrupted/grey, running=spinner) + duration; click → detail route. RunControl: model checkboxes (all default), task checkboxes (all default), Run button (disabled while active), Cancel button, progress `done/total` bound to SSE.
- [ ] **Step 1:** Scaffold, `npm i`, add vitest + @testing-library/svelte.
- [ ] **Step 2: Failing vitest**: Matrix renders chips from a canned matrix fixture; RunControl posts selected ids (mock fetch) and disables during active.
- [ ] **Step 3:** Implement components → tests PASS, `npm run build` clean.
- [ ] **Step 4: Commit** `feat: ui scaffold with matrix and run control`

### Task 8: Run detail (Transcript, FileTree, Preview) + history

**Files:**

- Create: `ui/src/lib/RunDetail.svelte`, `ui/src/lib/Transcript.svelte`, `ui/src/lib/FileTree.svelte`, `ui/src/lib/Preview.svelte`

**Interfaces:**

- Transcript props `{events: unknown[]}`: renders assistant text parts as markdown-ish `<pre class="text">`, tool events as collapsible `<details>` (summary: tool name; body: args + output JSON pretty-printed); unknown events skipped.
- FileTree props `{ref, files}`: nested list from flat paths; click loads file via api into `<pre><code>` viewer (no highlighting library — keep it plain).
- Preview props `{ref}`: `<iframe sandbox="allow-scripts" src=/api/runs/.../preview/index.html>` shown only when `index.html` exists in files.
- RunDetail: stats bar from Result (status, duration, messages, toolCalls, tokens), history `<select>` of timestamps (from history endpoint) that swaps the loaded run, check.log panel for check tasks, layout: stats / preview / transcript / files.
- [ ] **Step 1: Failing vitest**: Transcript groups fixture events (2 text, 1 tool) correctly; FileTree nests `a/b.txt`.
- [ ] **Step 2:** Implement → PASS, build clean.
- [ ] **Step 3: Commit** `feat: run detail with transcript, files, iframe preview`

### Task 9: Compare + NewTask + embed + Makefile chain

**Files:**

- Create: `ui/src/lib/Compare.svelte`, `ui/src/lib/NewTask.svelte`, `internal/server/embed.go`, `internal/server/embed_notag.go`
- Modify: `Makefile`, `internal/server/server.go`

**Interfaces:**

- Compare: task select + model multi-select; grid of columns per model: stats, Preview, final assistant message.
- NewTask: form fields mirroring Task; POST; on 400 shows error; on success routes home.
- Embed: `//go:build ui` file with `//go:embed all:ui_dist` serving SPA (fallback to index.html); notag build serves 404 JSON hint. Makefile: `make ui` builds Vite → copies `ui/dist` → `internal/server/ui_dist`; `make build` = ui + `go build -tags ui`; `make check` = `go vet ./... && go test ./... && (cd ui && npm test -- --run && npm run build)`.
- [ ] **Step 1:** Implement Compare/NewTask (+ vitest for NewTask validation error path) → PASS.
- [ ] **Step 2:** Embed files + Makefile; `make build` produces binary; manual smoke: `./opencode-bench` serves UI on 7777.
- [ ] **Step 3: Commit** `feat: compare view, task creation, embedded ui single binary`

### Task 10: Review task library (10 YAML files)

**Files:**

- Create: `tasks/checkers.yaml`, `tasks/tetris.yaml`, `tasks/markdown-editor.yaml`, `tasks/spreadsheet.yaml`, `tasks/flowchart-editor.yaml`, `tasks/data-dashboard.yaml`, `tasks/physics-sandbox.yaml`, `tasks/calendar.yaml`, `tasks/kanban.yaml`, `tasks/image-editor.yaml`

Each: `type: review`, `timeout_minutes: 45`, category per spec, and a prompt of 250–500 words that (a) opens with the verbatim static-app constraint block from Global Constraints, (b) enumerates every feature bullet listed for it in the spec's "Task library (initial)" section as explicit numbered requirements, (c) ends with "When finished, verify the app opens correctly from index.html." Exemplar (checkers) to copy structurally:

```yaml
id: checkers
title: "Checkers with AI"
category: games
type: review
timeout_minutes: 45
prompt: |
  Build a self-contained static web app: index.html entry point, no build
  step, no external network access (no CDNs, no fonts, no fetch). All code
  in plain HTML/CSS/JS files loaded from index.html.

  Implement a complete checkers game:
  1. 8x8 board, red vs black, red moves first. Click a piece, then a
     destination.
  2. Enforce ALL rules: diagonal moves on dark squares only, forced
     captures (if any capture exists you must capture), multi-jump chains
     in a single turn, kinging on the back rank, kings move both ways.
  3. Undo button reverting one full turn; move history panel in standard
     notation.
  4. Win/draw detection (no pieces, no legal moves).
  5. Single-player mode vs a minimax AI (selectable depth 1-5) with
     alpha-beta pruning; AI respects forced captures.
  6. Highlight selected piece, legal destinations, and last move.
  When finished, verify the app opens correctly from index.html.
```

- [ ] **Step 1:** Author all 10 files. **Step 2:** `go test ./internal/tasks/` still green; add a repo test `tasks_dir_test.go` in `internal/tasks` loading the real `../../tasks` dir asserting 15 tasks, 0 warnings (added in Task 11 — here assert ≥10, 0 warnings).
- [ ] **Step 3: Commit** `feat: ten complex visually-verifiable review tasks`

### Task 11: Check task library (5 YAML files)

**Files:**

- Create: `tasks/go-algorithms.yaml`, `tasks/python-expression-engine.yaml`, `tasks/csv-toolkit.yaml`, `tasks/http-api-contract.yaml`, `tasks/race-fix.yaml`

Each `type: check`, `timeout_minutes: 45`. The prompt states the deliverable and the exact commands the check will run; the check script is fully written in the YAML. Requirements per spec section "Check". Structural exemplar (race-fix, the trickiest — prompt embeds the racy code + test verbatim; check enforces test-file immutability by sha256):

```yaml
id: race-fix
title: "Fix the race without touching the test"
category: concurrency
type: check
timeout_minutes: 45
prompt: |
  In the working directory create a Go module "cache" with exactly these
  two files, then fix the data race in cache.go WITHOUT modifying
  cache_test.go in any way. go.mod: module cache, go 1.22.
  --- cache.go (as given, contains the race) ---
  <full ~40-line TTL cache with unsynchronized map access>
  --- cache_test.go (must remain byte-identical) ---
  <full test file exercising concurrent Get/Set, run with -race>
  All tests must pass with the race detector enabled.
check: |
  set -e
  cd "$(pwd)"
  echo "<sha256 of the exact cache_test.go text>  cache_test.go" | sha256sum -c -
  go test -race ./...
```

For `http-api-contract`, the check starts the built server on a free port in background, curls the contract (~20 assertions incl. 401 without JWT, 200+token flow, ETag→304, cursor pagination invariants, error codes), kills it. For `csv-toolkit` the check is a pure-bash assertion battery. For `go-algorithms` and `python-expression-engine` the check writes nothing: the prompt instructs the agent to place code so the shipped-in-prompt test files (embedded verbatim in prompt) compile against it; check runs `go vet ./... && go test -race ./...` / `python -m pytest -q`.

- [ ] **Step 1:** Author all 5 with complete embedded code/tests/scripts (compute the sha256 after writing the test text). **Step 2:** Update `tasks_dir_test.go` to assert 15/0. **Step 3:** Dry-run each check script's _failure_ path in an empty dir (all must fail cleanly, not hang).
- [ ] **Step 4: Commit** `feat: five scripted check benchmarks`

### Task 12: README + integration smoke

**Files:**

- Create: `README.md`; Modify: `Makefile` (target `smoke`)

- [ ] **Step 1:** README: what it is, `make build && ./opencode-bench`, adding a model (opencode config), adding a task (YAML schema), runs layout, caveat about `--auto`.
- [ ] **Step 2:** `make smoke`: starts binary, POSTs a batch of one trivial inline task (created via API: review task "create an index.html containing the text OPENCODE-BENCH-SMOKE") × first discovered model, polls until done, asserts workspace file exists. Manual/GPU target — not in `make check`.
- [ ] **Step 3:** Run `make check` (full gate) then `make smoke` once against a real model.
- [ ] **Step 4: Commit** `docs: readme + smoke target`

## Self-Review Notes

- Spec coverage: every spec section maps to a task (discovery→2, tasks→3, runner→5, store→4, API→6, UI→7-9, library→10-11, testing→throughout, error handling→5&6 tests). Restart/`interrupted` covered in Task 4. Cancel covered in 5&6.
- Type consistency: `Result`, `RunRef`, `FileEntry`, `Event`, `Task`, `Library`, `Model` defined once (Tasks 2-5) and consumed by name afterwards. Runner timeout seam locked as exported `Runner.Timeout`.
- No placeholders remain: the two YAML exemplars are structural templates; Tasks 10-11 enumerate exact per-file requirements via the spec's numbered feature lists, which travel with this plan.
