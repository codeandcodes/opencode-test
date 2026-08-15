# opencode-bench

Benchmark local LLMs through the [opencode](https://opencode.ai) agent
harness. Point it at the models in your opencode `llama-swap` provider, hit
Run, and compare how each model handles a suite of deliberately complex
coding tasks — visually for app-building tasks, scripted pass/fail for
benchmark tasks.

## How it works

- **Models** are discovered live from `~/.config/opencode/opencode.json`
  (`provider["llama-swap"].models`). Add a model there and it appears here
  on refresh — no config in this repo.
- **Tasks** are YAML files in `tasks/`. Two types:
  - `review` — open-ended app builds. Every review task must produce a
    self-contained static web app (`index.html`, no build step, no network),
    so the run viewer can render it in a sandboxed iframe for eyeball
    judgment.
  - `check` — tasks with a `check:` bash script run in the workspace after
    the agent finishes; exit 0 = pass.
- **Runs** execute `opencode run --dir <workspace> -m llama-swap/<model>
--format json --auto <prompt>` serially (one GPU), grouped by model to
  amortize llama-swap's model-swap cost. Everything lands under
  `runs/<task>/<model>/<timestamp>/`: the agent's workspace, the raw JSON
  event stream, stderr, check output, and `result.json`. Plain files — grep,
  diff, and delete at will.

## Usage

```bash
make build          # builds the Svelte UI and the Go binary (embedded UI)
./opencode-bench    # serves http://127.0.0.1:7777
```

Flags: `-listen`, `-opencode-config`, `-tasks`, `-runs`, `-opencode`
(binary path), `-idle-timeout` (minutes of event-stream silence before a
job is killed as stuck, default 10, 0 disables), `-llama-swap-config`
(path to llama-swap's YAML; when set,
each run snapshots the model's serving entry — quant, sampling flags,
context — into its `provenance.json`, and the matrix flags runs as stale
when a task's prompt changes after they ran).

Dev mode: `go run . ` for the API + `cd ui && npm run dev` for a hot-reload
UI proxied to it.

`make check` runs the full test gate (Go unit tests with the stubbed
opencode binary, vet, UI tests, UI build). `make smoke` runs one trivial
task against your first real model end-to-end — requires a working local
opencode + llama-swap.

## Adding a task

Drop `tasks/<id>.yaml`:

```yaml
id: my-task
title: "My Task"
category: games
type: review # or: check
timeout_minutes: 45
prompt: |
  ...the full prompt given to the agent...

# check: |            # check tasks only
#   go test ./...
```

or use the New Task form in the UI. Review prompts should demand a static
web app (see any existing review task for the constraint block to copy).

## Caveat

`opencode run --auto` auto-approves the agent's actions, including shell
commands, with your user's privileges — same trust level as using opencode
interactively, applied to whatever model you're benchmarking. Workspaces
isolate files, not permissions.
