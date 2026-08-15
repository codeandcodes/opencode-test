#!/usr/bin/env bash
# End-to-end smoke test: run one trivial task against the first discovered
# model through the real opencode + llama-swap stack. Requires GPU + opencode.
set -euo pipefail

BIN=${BIN:-./opencode-bench}
PORT=${PORT:-17788}
BASE="http://127.0.0.1:$PORT"
TMP=$(mktemp -d)
trap 'kill $SERVER_PID 2>/dev/null || true; rm -rf "$TMP"' EXIT

mkdir -p "$TMP/tasks" "$TMP/runs"
cat > "$TMP/tasks/smoke-hello.yaml" <<'EOF'
id: smoke-hello
title: "Smoke"
category: smoke
type: review
timeout_minutes: 10
prompt: |
  Create a single file named index.html whose body contains exactly the
  text OPENCODE-BENCH-SMOKE. Do nothing else.
EOF

"$BIN" -listen "127.0.0.1:$PORT" -tasks "$TMP/tasks" -runs "$TMP/runs" &
SERVER_PID=$!

for _ in $(seq 1 50); do
  curl -sf "$BASE/healthz" >/dev/null 2>&1 && break
  sleep 0.2
done

MODEL=$(curl -sf "$BASE/api/models" | python3 -c "import json,sys; print(json.load(sys.stdin)[0]['id'])")
echo "smoke: using model $MODEL"

curl -sf -X POST "$BASE/api/runs" -H 'Content-Type: application/json' \
  -d "{\"models\":[\"$MODEL\"],\"tasks\":[\"smoke-hello\"]}" >/dev/null

echo "smoke: waiting for run to finish (up to 12 min)..."
for _ in $(seq 1 144); do
  RUNNING=$(curl -sf "$BASE/api/runs" | python3 -c "import json,sys; print(json.load(sys.stdin)['active']['running'])")
  [ "$RUNNING" = "False" ] && break
  sleep 5
done

STATUS=$(curl -sf "$BASE/api/runs" | python3 -c "import json,sys; print(json.load(sys.stdin)['matrix']['smoke-hello']['$MODEL']['status'])")
echo "smoke: run status = $STATUS"
[ "$STATUS" = "done" ] || { echo "smoke: FAIL (status $STATUS)"; exit 1; }

if grep -rq "OPENCODE-BENCH-SMOKE" "$TMP"/runs/smoke-hello/*/*/workspace/; then
  echo "smoke: PASS"
else
  echo "smoke: FAIL (marker not found in workspace)"
  exit 1
fi
