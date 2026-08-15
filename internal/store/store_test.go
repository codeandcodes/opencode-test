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
