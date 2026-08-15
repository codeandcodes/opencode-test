package runner

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// ParseEvents tolerantly scans an opencode --format json event stream.
// Counting is heuristic by design: unknown/unparseable lines are skipped,
// so a schema drift degrades stats to zero instead of failing runs.
func ParseEvents(path string) (messages, toolCalls, tokensIn, tokensOut int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			continue
		}
		typ, _ := m["type"].(string)
		switch {
		case strings.Contains(typ, "tool"):
			toolCalls++
		case strings.Contains(typ, "message"):
			messages++
		}
		for _, key := range []string{"tokens", "usage"} {
			if u, ok := m[key].(map[string]any); ok {
				if v, ok := u["input"].(float64); ok {
					tokensIn += int(v)
				}
				if v, ok := u["output"].(float64); ok {
					tokensOut += int(v)
				}
			}
		}
	}
	return
}
