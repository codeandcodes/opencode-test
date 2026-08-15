package runner

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// EventStats aggregates what the opencode --format json event stream tells
// us about a run.
type EventStats struct {
	Messages        int // assistant steps (step_finish events)
	ToolCalls       int
	TokensIn        int
	TokensOut       int
	TokensReasoning int
	CacheRead       int
	// GenSeconds sums text/reasoning part time windows: active token
	// generation, excluding tool execution and model load.
	GenSeconds float64
}

// ParseEvents tolerantly scans an opencode event stream. The schema (as of
// opencode 1.x): every line is {"type":..., "timestamp":..., "part":{...}};
// token usage lives at part.tokens on step_finish events, and text /
// reasoning parts carry a part.time {start,end} generation window in ms.
// Unknown or unparseable lines are skipped so schema drift degrades stats
// to zero instead of failing runs.
func ParseEvents(path string) EventStats {
	var s EventStats
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			continue
		}
		typ, _ := m["type"].(string)
		part, _ := m["part"].(map[string]any)

		switch {
		case strings.Contains(typ, "tool"):
			s.ToolCalls++
		case typ == "step_finish":
			s.Messages++
			if tk, ok := part["tokens"].(map[string]any); ok {
				s.TokensIn += intAt(tk, "input")
				s.TokensOut += intAt(tk, "output")
				s.TokensReasoning += intAt(tk, "reasoning")
				if cache, ok := tk["cache"].(map[string]any); ok {
					s.CacheRead += intAt(cache, "read")
				}
			}
		}

		if pt, _ := part["type"].(string); pt == "text" || pt == "reasoning" {
			if tm, ok := part["time"].(map[string]any); ok {
				start, ok1 := tm["start"].(float64)
				end, ok2 := tm["end"].(float64)
				if ok1 && ok2 && end > start {
					s.GenSeconds += (end - start) / 1000
				}
			}
		}
	}
	return s
}

func intAt(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
