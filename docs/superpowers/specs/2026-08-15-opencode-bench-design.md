# opencode-bench — local model benchmark harness

Date: 2026-08-15
Status: approved (design discussed and accepted in-session)

## Purpose

A local web application that benchmarks local LLMs through the opencode agent
harness. One click runs a suite of complex coding tasks against any or all
models configured in opencode's `llama-swap` provider, capturing full agent
traces and outputs. Review tasks are judged by a human comparing outputs
side-by-side (every review task builds a static web app so results are
verifiable visually in an embedded preview); check tasks are scored
pass/fail by a verification script.

Primary workflows:

1. New model released → add it to llama-swap + opencode config (existing
   workflow) → it appears in opencode-bench automatically → run it against
   all tasks → compare against other models.
2. New prompt idea → drop a YAML file in `tasks/` (or use the new-task form)
   → run it against all models.

## Constraints

- Stack: Go (single binary, embedded UI) + Svelte 5/Vite UI. Mirrors
  llama-swap's stack.
- Models come only from the `llama-swap` provider in
  `~/.config/opencode/opencode.json`. No separate model registry.
- One GPU: runs execute serially, grouped by model to amortize llama-swap's
  model-swap cost.
- Results are plain files under `runs/`. No database.
- `opencode run --auto` executes agent-chosen shell commands unsandboxed
  (same trust level as interactive opencode use). Accepted for local use.

## Components

### 1. Model discovery (`internal/config`)

Parses `~/.config/opencode/opencode.json` (path overridable by flag),
returns the model IDs and display names under `provider["llama-swap"].models`.
Read at request time — no caching — so config edits appear on refresh.

### 2. Task library (`internal/tasks` + `tasks/*.yaml`)

One YAML file per task:

```yaml
id: tetris # unique, filename must match <id>.yaml
title: "Tetris"
category: algorithms # free-form grouping label
type: review # review | check
timeout_minutes: 30 # default 30
prompt: |
  ...multi-paragraph prompt...
check: | # check tasks only: bash, run in workspace, exit 0 = pass
  ...script...
```

Validation at load: unique ids, type∈{review,check}, check present iff
type=check, prompt non-empty. Invalid task files are reported, not fatal.

Review tasks all require the agent to produce a self-contained static web
app (`index.html` entry point, no build step, no external network) so the
UI's iframe preview always works.

### 3. Runner (`internal/runner`)

- Job = (task, model). A run request expands {models}×{tasks} into jobs,
  ordered so all jobs for one model complete before the next model starts.
- One job at a time, one active batch at a time (409 if a batch is running).
- Per job:
  1. Create `runs/<taskID>/<modelID>/<RFC3339 timestamp>/workspace/`.
  2. Exec `opencode run --dir <workspace> -m llama-swap/<model>
--format json --auto <prompt>`, stdout streamed to `events.jsonl`,
     stderr to `stderr.log`, killed at task timeout (status `timeout`).
  3. For check tasks: run the check script with `bash` in the workspace
     (own timeout, 5 min), capture output to `check.log`, exit 0 → `pass`,
     else `fail`. Review tasks get status `done` (await human judgment).
  4. Write `result.json`: task, model, status
     (`done|pass|fail|timeout|error`), started/finished timestamps,
     duration, message/tool-call/token counts parsed from events.jsonl
     (best-effort; zero values when events are unparseable).
- Batch progress (current job, queue position, per-job status) is held in
  memory and streamed over SSE; it is also recoverable from `result.json`
  files after restart. A batch can be cancelled (current job's process
  group is killed, remaining jobs skipped).

### 4. Store (`internal/store`)

Filesystem layout is the source of truth:

```
runs/<taskID>/<modelID>/<timestamp>/
  workspace/      # what the agent built
  events.jsonl    # raw opencode JSON event stream
  stderr.log
  check.log       # check tasks only
  result.json
```

Store lists runs by scanning this tree, exposes latest run per
(task, model), full history per cell, and file access inside a run's
workspace (path-traversal-safe: resolved paths must stay inside the run dir).

### 5. HTTP API (`internal/server`)

- `GET  /api/models` — discovered models
- `GET  /api/tasks` — task library (id, title, category, type, timeout;
  prompt on detail endpoint `GET /api/tasks/{id}`)
- `POST /api/tasks` — create task file from JSON body (writes `tasks/<id>.yaml`)
- `POST /api/runs` — `{models: [...], tasks: [...]}` → starts batch, 409 if busy
- `DELETE /api/runs/active` — cancel running batch
- `GET  /api/runs` — matrix: latest result per (task, model)
- `GET  /api/runs/{task}/{model}` — run history for a cell
- `GET  /api/runs/{task}/{model}/{ts}` — result.json + parsed transcript
- `GET  /api/runs/{task}/{model}/{ts}/files[/{path}]` — workspace tree/file
- `GET  /api/runs/{task}/{model}/{ts}/preview/{path}` — serves workspace
  files with correct MIME types for the iframe preview (CSP: no external
  network)
- `GET  /api/events` — SSE batch progress
- `GET  /` — embedded Svelte UI (go:embed of `ui/dist`)

Listen address flag `-listen`, default `127.0.0.1:7777`.

### 6. UI (Svelte 5 + Vite, `ui/`)

- **Matrix** (home): task rows × model columns. Cells: pass/fail badge
  (check), duration + review link (review), spinner for the active job.
  Buttons: select models/tasks, Run, Cancel. Live via SSE.
- **Run detail**: transcript rendered from events.jsonl (assistant text,
  collapsible tool calls with args/output), stats bar (duration, turns,
  tool calls, tokens), workspace file tree with code viewer, and for
  review tasks a sandboxed iframe preview of `index.html`. History
  dropdown to view older runs of the same cell.
- **Compare**: pick one task + N models; side-by-side columns of stats,
  final outputs, and preview iframes.
- **New task**: form (id, title, category, type, timeout, prompt, check)
  posting to `/api/tasks`.

## Error handling

- opencode exits non-zero or events.jsonl is empty → status `error`,
  stderr.log surfaced in run detail.
- Model not in llama-swap config → job fails fast with a clear error.
- Server restart mid-batch: in-memory batch is lost; completed runs remain
  on disk; the interrupted job's dir has no result.json and is shown as
  `interrupted`; the batch is not auto-resumed.
- Task YAML invalid → excluded from library, listed in a `GET /api/tasks`
  warnings field.

## Testing

- Go: unit tests for config parsing, task loading/validation, store
  scanning and path-safety, result.json parsing, and the runner driven by
  a stub `opencode` script (bash fake emitting canned JSON events; also
  variants that hang → timeout path, exit non-zero → error path,
  write files → check path). `go test ./...` needs no GPU and no real
  opencode.
- UI: vitest component tests for matrix state and transcript rendering
  from a canned events fixture.
- One optional integration script (Makefile target) that runs a single
  trivial task against a real local model end-to-end.

## Task library (initial)

Review (all produce static web apps, all deliberately complex):

1. checkers — full rules: forced captures, multi-jump, kinging, undo,
   minimax AI opponent with selectable depth
2. tetris — SRS rotation + wall kicks, hold, ghost, 7-bag, combo scoring,
   gravity curve, persisted high scores
3. markdown-editor — hand-rolled parser (tables, nested lists, fenced code
   with JS tokenizer-based highlighting), synced split pane, versioned
   autosave
4. spreadsheet — formula engine: cell/range refs, SUM/AVG/IF, dependency
   graph, cycle detection, auto-recalc, fill handle
5. flowchart-editor — draggable nodes, auto-routing connectors, undo/redo,
   SVG export, keyboard shortcuts
6. data-dashboard — parse embedded messy CSV; hand-rolled SVG line/bar/
   scatter charts with tooltips, brushing, cross-chart filtering
7. physics-sandbox — 2D elastic collisions, gravity, drag-to-throw,
   parameter controls, smooth canvas loop
8. calendar — month/week views, drag-to-create/resize, recurring rules
   (weekly, monthly-by-weekday) correct across DST, localStorage
9. kanban — drag-and-drop cards, WIP limits, filtering, undo, persistence
10. image-editor — load/crop/rotate, per-pixel canvas filters via
    convolution kernels, non-destructive undo stack

Check (scripted pass/fail):

11. go-algorithms — interval-set arithmetic, topo sort with deterministic
    tie-breaking, TTL-LRU, glob matcher, token bucket; check ships tests,
    runs `go test`, `go vet`, `-race`
12. python-expression-engine — tokenizer + evaluator (precedence, parens,
    unary, variables, functions); pytest suite of edge cases
13. csv-toolkit — select/filter/join/aggregate CLI honoring RFC 4180;
    bash contract asserts stdout/stderr/exit codes across ~30 invocations
14. http-api-contract — JWT auth, ETag/304s, cursor pagination, correct
    error codes; curl-based contract script
15. race-fix — provided racy concurrent cache + failing `-race` test;
    fix without touching the test file (hash-enforced), all tests green

## Out of scope (deliberate)

Aggregate scoring beyond per-model pass counts, LLM-as-judge, fixture
repos (schema leaves room: a future `fixture:` key), auth, multi-machine,
parallel GPU runs, run diffing between models beyond side-by-side view.
