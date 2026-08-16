package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewRunDirAndRoundTrip(t *testing.T) {
	st := New(t.TempDir())
	ref, ws, err := st.NewRunDir("tetris", "model-a")
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(ws); err != nil || !fi.IsDir() {
		t.Fatalf("workspace not created: %v", err)
	}
	res := Result{Task: "tetris", Model: "model-a", Status: "done",
		StartedAt: time.Now().UTC().Truncate(time.Second), DurationSec: 12.5,
		Messages: 3, ToolCalls: 2, TokensIn: 100, TokensOut: 200, Timestamp: ref.Timestamp}
	if err := st.WriteResult(ref, res); err != nil {
		t.Fatal(err)
	}
	got, err := st.ReadResult(ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "done" || got.Messages != 3 || got.Timestamp != ref.Timestamp {
		t.Fatalf("round trip: %+v", got)
	}
}

func TestLatestAndInterrupted(t *testing.T) {
	st := New(t.TempDir())
	ref1, _, _ := st.NewRunDir("tetris", "model-a")
	st.WriteResult(ref1, Result{Task: "tetris", Model: "model-a", Status: "fail", Timestamp: ref1.Timestamp})
	time.Sleep(1100 * time.Millisecond) // timestamps have second resolution
	ref2, _, _ := st.NewRunDir("tetris", "model-a")
	st.WriteResult(ref2, Result{Task: "tetris", Model: "model-a", Status: "pass", Timestamp: ref2.Timestamp})
	// interrupted: dir with no result.json
	if _, _, err := st.NewRunDir("kanban", "model-a"); err != nil {
		t.Fatal(err)
	}

	m, err := st.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if got := m["tetris"]["model-a"].Status; got != "pass" {
		t.Fatalf("latest tetris = %q, want pass (newest)", got)
	}
	if got := m["kanban"]["model-a"].Status; got != "interrupted" {
		t.Fatalf("kanban = %q, want interrupted", got)
	}
}

func TestHistoryNewestFirst(t *testing.T) {
	st := New(t.TempDir())
	ref1, _, _ := st.NewRunDir("tetris", "m")
	st.WriteResult(ref1, Result{Status: "fail", Timestamp: ref1.Timestamp})
	time.Sleep(1100 * time.Millisecond)
	ref2, _, _ := st.NewRunDir("tetris", "m")
	st.WriteResult(ref2, Result{Status: "pass", Timestamp: ref2.Timestamp})
	h, err := st.History("tetris", "m")
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 2 || h[0].Status != "pass" || h[1].Status != "fail" {
		t.Fatalf("history = %+v", h)
	}
}

func TestNewRunDirCollisionSafe(t *testing.T) {
	st := New(t.TempDir())
	refs := map[string]bool{}
	for i := 0; i < 3; i++ { // same second: must yield distinct dirs
		ref, _, err := st.NewRunDir("tetris", "m")
		if err != nil {
			t.Fatal(err)
		}
		if refs[ref.Timestamp] {
			t.Fatalf("duplicate timestamp %q", ref.Timestamp)
		}
		refs[ref.Timestamp] = true
		st.WriteResult(ref, Result{Status: "done", Timestamp: ref.Timestamp})
	}
	h, err := st.History("tetris", "m")
	if err != nil || len(h) != 3 {
		t.Fatalf("history = %d (%v), want 3", len(h), err)
	}
}

func TestAggregate(t *testing.T) {
	now := time.Now()
	h := []Result{
		{Status: "pass", GenSeconds: 10, TokensOut: 500, FinishedAt: now},
		{Status: "fail", GenSeconds: 10, TokensOut: 800, FinishedAt: now},
		{Status: "pass", GenSeconds: 20, TokensOut: 400, FinishedAt: now},
		{Status: "error", FinishedAt: now}, // not a sample
		{Status: "done", FinishedAt: now, Verdict: &Verdict{Verdict: "good"}},
		{Status: "done", FinishedAt: now, Verdict: &Verdict{Verdict: "bad"}},
		{Status: "done", FinishedAt: now},
		{Status: "done", FinishedAt: now, Verdict: &Verdict{Rating: 8}},
		{Status: "done", FinishedAt: now, Verdict: &Verdict{Rating: 3}},
	}
	agg := Aggregate(h)
	if agg.Samples != 8 { // pass+fail+pass+done×5
		t.Fatalf("samples = %d", agg.Samples)
	}
	if agg.Passes != 2 || agg.Fails != 1 || agg.Dones != 5 {
		t.Fatalf("counts: %+v", agg)
	}
	if agg.VerdictGood != 1 || agg.VerdictBad != 1 {
		t.Fatalf("verdicts: %+v", agg)
	}
	if agg.RatingCount != 2 || agg.RatingAvg < 5.49 || agg.RatingAvg > 5.51 {
		t.Fatalf("ratings: count=%d avg=%v want 2/5.5", agg.RatingCount, agg.RatingAvg)
	}
	// tps samples: 50, 80, 20 -> median 50
	if agg.MedianTps < 49.9 || agg.MedianTps > 50.1 {
		t.Fatalf("median tps = %v", agg.MedianTps)
	}
	if empty := Aggregate(nil); empty.Samples != 0 || empty.MedianTps != 0 {
		t.Fatalf("empty agg: %+v", empty)
	}
}

func TestVerdicts(t *testing.T) {
	st := New(t.TempDir())
	ref, _, _ := st.NewRunDir("tetris", "m")
	st.WriteResult(ref, Result{Task: "tetris", Model: "m", Status: "done", Timestamp: ref.Timestamp})

	if err := st.WriteVerdict(ref, Verdict{Verdict: "good", Note: "solid SRS"}); err != nil {
		t.Fatal(err)
	}
	m, _ := st.Latest()
	v := m["tetris"]["m"].Verdict
	if v == nil || v.Verdict != "good" || v.Note != "solid SRS" || v.At.IsZero() {
		t.Fatalf("verdict not attached: %+v", v)
	}
	h, _ := st.History("tetris", "m")
	if h[0].Verdict == nil {
		t.Fatal("verdict missing from history")
	}

	if err := st.WriteVerdict(ref, Verdict{Verdict: "meh"}); err == nil {
		t.Fatal("invalid verdict accepted")
	}
	// ratings: 1-10 valid, others rejected
	if err := st.WriteVerdict(ref, Verdict{Rating: 7, Note: "solid but no undo"}); err != nil {
		t.Fatalf("rating verdict rejected: %v", err)
	}
	m2, _ := st.Latest()
	if v := m2["tetris"]["m"].Verdict; v == nil || v.Rating != 7 {
		t.Fatalf("rating not attached: %+v", v)
	}
	for _, bad := range []int{-1, 11, 100} {
		if err := st.WriteVerdict(ref, Verdict{Rating: bad}); err == nil {
			t.Fatalf("rating %d accepted", bad)
		}
	}
	if err := st.WriteVerdict(ref, Verdict{}); err == nil {
		t.Fatal("empty verdict accepted")
	}
	if err := st.ClearVerdict(ref); err != nil {
		t.Fatal(err)
	}
	m, _ = st.Latest()
	if m["tetris"]["m"].Verdict != nil {
		t.Fatal("verdict not cleared")
	}
	if err := st.ClearVerdict(ref); err != nil {
		t.Fatalf("clearing absent verdict should be idempotent: %v", err)
	}
}

func TestSafePath(t *testing.T) {
	st := New(t.TempDir())
	ref, ws, _ := st.NewRunDir("tetris", "m")
	os.MkdirAll(filepath.Join(ws, "sub"), 0o755)
	os.WriteFile(filepath.Join(ws, "sub", "file.txt"), []byte("hi"), 0o644)

	if _, err := st.SafePath(ref, "sub/file.txt"); err != nil {
		t.Fatalf("legit path rejected: %v", err)
	}
	for _, bad := range []string{"../result.json", "../../etc/passwd", "/etc/passwd", "sub/../../escape"} {
		if _, err := st.SafePath(ref, bad); err == nil {
			t.Fatalf("path %q not rejected", bad)
		}
	}
}

func TestListFiles(t *testing.T) {
	st := New(t.TempDir())
	ref, ws, _ := st.NewRunDir("tetris", "m")
	os.MkdirAll(filepath.Join(ws, "css"), 0o755)
	os.MkdirAll(filepath.Join(ws, ".git", "objects"), 0o755)
	os.WriteFile(filepath.Join(ws, ".git", "config"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(ws, "index.html"), []byte("<html>"), 0o644)
	os.WriteFile(filepath.Join(ws, "css", "app.css"), []byte("body{}"), 0o644)
	files, err := st.ListFiles(ref)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range files {
		if !f.Dir {
			paths = append(paths, f.Path)
		}
	}
	if len(paths) != 2 || paths[0] != "css/app.css" || paths[1] != "index.html" {
		t.Fatalf("paths = %v", paths)
	}
}
