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
	// GenSeconds is model-active generation time: the sum of step windows
	// (step_start to step_finish) minus tool execution windows and minus
	// LoadSeconds. It covers generation of all parts plus later-step
	// prompt processing.
	GenSeconds float64
	// LoadSeconds is the first-token wait: first step_start to the first
	// part's generation start — model swap/load plus the initial prefill.
	LoadSeconds float64
}

// ParseEvents tolerantly scans an opencode event stream. The schema (as of
// opencode 1.x): every line is {"type":..., "timestamp":..., "part":{...}};
// token usage lives at part.tokens on step_finish events; tool parts carry
// an execution window at part.state.time {start,end} in ms. Unknown or
// unparseable lines are skipped so schema drift degrades stats to zero
// instead of failing runs.
func ParseEvents(path string) EventStats {
	var s EventStats
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	var stepStart, firstStepStart, firstPartStart float64
	var toolSeconds float64
	partStart := func(part map[string]any, key string) float64 {
		tm, ok := part[key].(map[string]any)
		if !ok {
			return 0
		}
		v, _ := tm["start"].(float64)
		return v
	}
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			continue
		}
		typ, _ := m["type"].(string)
		part, _ := m["part"].(map[string]any)
		ts, _ := m["timestamp"].(float64)

		switch {
		case strings.Contains(typ, "tool"):
			s.ToolCalls++
			if state, ok := part["state"].(map[string]any); ok {
				toolSeconds += windowSeconds(state["time"])
				if v := partStart(state, "time"); v > 0 && (firstPartStart == 0 || v < firstPartStart) {
					firstPartStart = v
				}
			} else {
				toolSeconds += windowSeconds(part["time"])
			}
		case typ == "step_start":
			stepStart = ts
			if firstStepStart == 0 {
				firstStepStart = ts
			}
		case typ == "step_finish":
			s.Messages++
			if stepStart > 0 && ts > stepStart {
				s.GenSeconds += (ts - stepStart) / 1000
			}
			stepStart = 0
			if tk, ok := part["tokens"].(map[string]any); ok {
				s.TokensIn += intAt(tk, "input")
				s.TokensOut += intAt(tk, "output")
				s.TokensReasoning += intAt(tk, "reasoning")
				if cache, ok := tk["cache"].(map[string]any); ok {
					s.CacheRead += intAt(cache, "read")
				}
			}
		default:
			if v := partStart(part, "time"); v > 0 && (firstPartStart == 0 || v < firstPartStart) {
				firstPartStart = v
			}
		}
	}
	if firstStepStart > 0 && firstPartStart > firstStepStart {
		s.LoadSeconds = (firstPartStart - firstStepStart) / 1000
	}
	s.GenSeconds -= toolSeconds + s.LoadSeconds
	if s.GenSeconds < 0 {
		s.GenSeconds = 0
	}
	return s
}

// TailEvents returns the last n valid JSON event lines of an event stream.
func TailEvents(path string, n int) []json.RawMessage {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	var tail []json.RawMessage
	for sc.Scan() {
		if !json.Valid(sc.Bytes()) {
			continue
		}
		line := make([]byte, len(sc.Bytes()))
		copy(line, sc.Bytes())
		tail = append(tail, line)
		if len(tail) > n {
			tail = tail[1:]
		}
	}
	return tail
}

func windowSeconds(v any) float64 {
	tm, ok := v.(map[string]any)
	if !ok {
		return 0
	}
	start, ok1 := tm["start"].(float64)
	end, ok2 := tm["end"].(float64)
	if !ok1 || !ok2 || end <= start {
		return 0
	}
	return (end - start) / 1000
}

func intAt(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
