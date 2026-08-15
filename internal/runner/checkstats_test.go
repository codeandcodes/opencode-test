package runner

import "testing"

func TestParseCheckLog(t *testing.T) {
	cases := []struct {
		name           string
		log            string
		passed, failed int
		parsed         bool
	}{
		{"pytest all pass", "........\n9 passed in 0.07s\n", 9, 0, true},
		{"pytest mixed", "FAILED test_engine.py::test_deep_nesting - boom\n1 failed, 8 passed in 0.11s\n", 8, 1, true},
		{"go test -v", "=== RUN   TestA\n--- PASS: TestA (0.00s)\n=== RUN   TestB\n--- FAIL: TestB (0.01s)\n--- PASS: TestC (0.00s)\nFAIL\nexit status 1\n", 2, 1, true},
		{"bash battery failures", "FAIL select-basic\nFAIL agg-sum\n2 assertion(s) failed\n", 0, 2, true},
		{"bash battery success", "all assertions passed\n", 0, 0, true},
		{"unrecognized", "compilation error: blah\n", 0, 0, false},
		{"empty", "", 0, 0, false},
	}
	for _, tc := range cases {
		p, f, ok := ParseCheckLog(tc.log)
		if p != tc.passed || f != tc.failed || ok != tc.parsed {
			t.Errorf("%s: got passed=%d failed=%d parsed=%v, want %d/%d/%v",
				tc.name, p, f, ok, tc.passed, tc.failed, tc.parsed)
		}
	}
}
