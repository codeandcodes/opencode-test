#!/usr/bin/env bash
# Safe deploy: refuses to restart the server while a batch is running.
# Usage: scripts/deploy.sh [extra server flags...]
set -euo pipefail

BASE="${BASE:-http://127.0.0.1:7777}"

if curl -sf "$BASE/api/runs" -o /tmp/opencode-bench-active.json 2>/dev/null; then
  RUNNING=$(python3 -c "import json; print(json.load(open('/tmp/opencode-bench-active.json'))['active']['running'])")
  if [ "$RUNNING" = "True" ]; then
    echo "REFUSING to deploy: a batch is running. Wait or cancel it first." >&2
    exit 1
  fi
fi

make build
pkill -TERM -x opencode-bench 2>/dev/null || true
sleep 1
nohup ./opencode-bench "$@" > /dev/null 2>&1 &
sleep 1
curl -sf "$BASE/healthz" && echo " deployed"
