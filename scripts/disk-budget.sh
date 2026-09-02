#!/usr/bin/env bash
# Report LLM bench storage against the 1TB budget and list eviction candidates.
# Image/video models are outside the budget and never counted or proposed.
set -euo pipefail

HUB="$HOME/.cache/huggingface/hub"
BUDGET_GB=1024
# Non-bench (image/video/audio) dirs excluded from the budget, one pattern per line.
EXCLUDE='Wan-AI--|FLUX|Qwen-Image|ComfyUI|stable-diffusion|whisper'

total_kb=0
listing=$(mktemp)
trap 'rm -f "$listing"' EXIT

while read -r kb path; do
  name="${path##*/models--}"
  if echo "$name" | grep -qE "$EXCLUDE"; then continue; fi
  total_kb=$((total_kb + kb))
  printf '%12d %s\n' "$kb" "$name" >> "$listing"
done < <(du -s "$HUB"/models--* 2>/dev/null)

# ~/models counts too (raw GGUFs outside the HF cache)
if [ -d "$HOME/models" ]; then
  kb=$(du -s "$HOME/models" | awk '{print $1}')
  total_kb=$((total_kb + kb))
  printf '%12d %s\n' "$kb" "(~/models)" >> "$listing"
fi

total_gb=$((total_kb / 1048576))
echo "LLM bench storage: ${total_gb} GB / ${BUDGET_GB} GB budget"
if [ "$total_gb" -gt "$BUDGET_GB" ]; then
  echo "OVER BUDGET by $((total_gb - BUDGET_GB)) GB — eviction needed (see PIPELINE.md order)"
else
  echo "Headroom: $((BUDGET_GB - total_gb)) GB"
fi
echo
echo "Largest bench models:"
sort -rn "$listing" | head -20 | awk '{printf "  %7.1f GB  %s\n", $1/1048576, $2}'
