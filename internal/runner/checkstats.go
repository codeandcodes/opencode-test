package runner

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	pytestRe    = regexp.MustCompile(`(?:(\d+) failed, )?(\d+) passed`)
	assertionRe = regexp.MustCompile(`(\d+) assertion\(s\) failed`)
)

// ParseCheckLog extracts pass/fail counts from a check script's output.
// Recognizes pytest summaries, `go test -v` result lines, and this repo's
// bash assertion batteries. parsed=false when no known format matched;
// passed may be 0 with parsed=true when the format only reports failures.
func ParseCheckLog(log string) (passed, failed int, parsed bool) {
	// go test -v
	goPass := strings.Count(log, "--- PASS:")
	goFail := strings.Count(log, "--- FAIL:")
	if goPass+goFail > 0 {
		return goPass, goFail, true
	}
	// pytest summary (use the last match: the summary line)
	if ms := pytestRe.FindAllStringSubmatch(log, -1); len(ms) > 0 {
		m := ms[len(ms)-1]
		if m[1] != "" {
			failed, _ = strconv.Atoi(m[1])
		}
		passed, _ = strconv.Atoi(m[2])
		return passed, failed, true
	}
	// bash assertion batteries
	if m := assertionRe.FindStringSubmatch(log); m != nil {
		failed, _ = strconv.Atoi(m[1])
		return 0, failed, true
	}
	if strings.Contains(log, "all assertions passed") {
		return 0, 0, true
	}
	return 0, 0, false
}
