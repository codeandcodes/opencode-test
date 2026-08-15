#!/usr/bin/env bash
# Fake `opencode run` for tests, emitting the real opencode --format json
# event schema. Behavior selected by $STUB_MODE:
#   ok   - emit 4 realistic JSON event lines, create hello.txt in --dir
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
    echo '{"type":"step_start","timestamp":1000,"part":{"type":"step-start"}}'
    echo '{"type":"text","timestamp":1500,"part":{"type":"text","text":"writing the file","time":{"start":1000,"end":3000}}}'
    echo '{"type":"tool","timestamp":1600,"part":{"type":"tool","tool":"write","state":{"status":"completed"}}}'
    echo '{"type":"step_finish","timestamp":2000,"part":{"type":"step-finish","reason":"stop","tokens":{"total":350,"input":100,"output":200,"reasoning":50,"cache":{"write":0,"read":40}}}}'
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
