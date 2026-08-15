#!/usr/bin/env bash
# Fake `opencode run` for tests. Behavior selected by $STUB_MODE:
#   ok   - emit 3 JSON event lines, create hello.txt in the --dir workspace
#   fail - exit 3 with stderr
#   hang - sleep far beyond any test timeout
set -u

dir=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "--dir" ]; then dir="$arg"; fi
  prev="$arg"
done

case "${STUB_MODE:-ok}" in
  ok)
    echo '{"type":"message.updated","role":"assistant"}'
    echo '{"type":"tool.completed","tool":"write","args":{"path":"hello.txt"}}'
    echo '{"type":"message.completed","tokens":{"input":100,"output":200}}'
    [ -n "$dir" ] && echo "hello" > "$dir/hello.txt"
    exit 0
    ;;
  fail)
    echo "stub: simulated failure" >&2
    exit 3
    ;;
  hang)
    sleep 300
    ;;
esac
