# Research Bench Pipeline

Standing pipeline for evaluating new local coding models as they are released.
This repo is the durable home: every model that passes through leaves a report
in `reports/`, a row on the leaderboard, and raw runs on the bench host.

## Cycle (per new model)

1. **Discover** — watch for new model releases worth benching: HF trending +
   org pages (Qwen, Meta, Google, Mistral, DeepSeek, THUDM/Z.ai, OpenAI-oss,
   NVIDIA, Moonshot), r/LocalLLaMA, X. Candidate filter: coding-capable,
   runnable on a single 96 GB card (GGUF ≤ ~90 GB weights or quantizable to
   fit), llama.cpp support merged or imminent.
2. **Intake** — download the best-fitting unsloth/bartowski quant to the HF
   cache; add a llama-swap entry (`config.local.yaml`) using the model card's
   recommended sampling; verify llama.cpp build support (rebuild if the
   architecture needs a newer build; back up `build/bin` first); smoke-test one
   completion through llama-swap; register the model in
   `~/.config/opencode/opencode.json`.
3. **Bench** — deploy opencode-bench (only via `scripts/deploy.sh`, which
   refuses while a batch is live) and run the full 20-task suite. Check tasks
   auto-score. Thinking-toggle models get two entries (think/fast) like
   Qwen3.8-27B.
4. **Grade** — LLM auto-rating of the 10 review tasks against the in-app
   rubric (`/api/rubric`):
   - open each produced app in a browser (Playwright), screenshot, and
     exercise the core interactions the task demands;
   - read the produced code for structural quality and correctness;
   - assign 1–10 per the rubric, POST via the verdict API with note prefix
     `[auto:claude]` so machine ratings are distinguishable from human ones;
   - never overwrite a human rating.
5. **Report** — write `reports/<model-slug>.md` (rendered in-app under the
   **Reports** tab, served from `/api/reports`): exact served configuration,
   composite score + leaderboard position, per-task table, generation
   statistics (tokens in/out, decode time, first-token, wall), 3–5 findings
   with screenshots in `reports/img/`, and a **community-signal section**
   (what X/Reddit/blogs are saying — search, quote, link). Commit and push.
6. **Budget** — keep LLM bench storage ≤ **1 TB** (image/video models — Wan,
   FLUX, Qwen-Image, ComfyUI — are outside this budget and never touched).
   Run `scripts/disk-budget.sh` each cycle. When over budget, evict in this
   order, and record every eviction in the cycle's report:
   1. duplicate quants of the same checkpoint (keep the benched one),
   2. F16/BF16 originals where a benched quant exists,
   3. raw safetensors with a GGUF equivalent,
   4. lowest-leaderboard models absent from the active llama-swap fleet.
      Deleting anything the current `config.local.yaml` references requires
      removing the config entry in the same change, and gets flagged to the
      human in the report.

## Grading protocol notes

- The rubric is the single standard (also embedded at
  `internal/server/rubric.json`). Band meanings: 1–2 non-functional,
  3 barely, 4–5 semi-functional, 6–7 functional with workarounds,
  8–10 fully functional with ascending UI quality.
- Auto-ratings are advisory until spot-checked: the human can re-rate any
  cell; human ratings always win.
- Comparisons matter more than absolutes — when grading a new model's app,
  open one known-anchor app of the same task (e.g. the current leaderboard
  leader's) to calibrate before scoring.

## Operational guardrails (learned the hard way)

- Deploy only through `scripts/deploy.sh` — it refuses while a batch runs.
- Check output goes to files, never pipes (pipe-inheritance wedge, fixed).
- Workspaces are git-anchored; never run tasks against a harness without it.
- `pkill -x opencode-bench`, never `pkill -f`.
- llama-swap reload is SIGHUP to the systemd user service
  (`llama-swap-linux-amd64`); config lives at
  `~/workspace/llama-swap/config.local.yaml`.
- Before replacing the llama.cpp binary, back up `build/bin` and re-smoke an
  existing fleet model with the new build.
