package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTask(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const validReview = `id: tetris
title: "Tetris"
category: games
type: review
timeout_minutes: 45
prompt: |
  Build tetris.
`

const validCheck = `id: go-algo
title: "Go algorithms"
category: backend
type: check
prompt: |
  Implement the functions.
check: |
  go test ./...
`

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	writeTask(t, dir, "tetris.yaml", validReview)
	writeTask(t, dir, "go-algo.yaml", validCheck)
	lib, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Warnings) != 0 {
		t.Fatalf("warnings: %v", lib.Warnings)
	}
	if len(lib.Tasks) != 2 {
		t.Fatalf("got %d tasks", len(lib.Tasks))
	}
	// reviews sort before checks
	if lib.Tasks[0].ID != "tetris" || lib.Tasks[1].ID != "go-algo" {
		t.Fatalf("order: %s, %s", lib.Tasks[0].ID, lib.Tasks[1].ID)
	}
	if lib.Tasks[0].TimeoutMinutes != 45 {
		t.Fatalf("timeout = %d", lib.Tasks[0].TimeoutMinutes)
	}
	if lib.Tasks[1].TimeoutMinutes != 30 { // default applied
		t.Fatalf("default timeout = %d", lib.Tasks[1].TimeoutMinutes)
	}
}

func TestLoadInvalidFilesBecomeWarnings(t *testing.T) {
	dir := t.TempDir()
	writeTask(t, dir, "tetris.yaml", validReview)
	writeTask(t, dir, "nocheck.yaml", "id: nocheck\ntitle: x\ntype: check\nprompt: p\n")
	writeTask(t, dir, "badtype.yaml", "id: badtype\ntitle: x\ntype: banana\nprompt: p\n")
	writeTask(t, dir, "mismatch.yaml", "id: other\ntitle: x\ntype: review\nprompt: p\n")
	writeTask(t, dir, "noprompt.yaml", "id: noprompt\ntitle: x\ntype: review\n")
	lib, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Tasks) != 1 || lib.Tasks[0].ID != "tetris" {
		t.Fatalf("tasks = %+v", lib.Tasks)
	}
	if len(lib.Warnings) != 4 {
		t.Fatalf("warnings = %v", lib.Warnings)
	}
	for _, w := range lib.Warnings {
		if !strings.Contains(w, ".yaml") {
			t.Fatalf("warning lacks filename: %q", w)
		}
	}
}

func TestLoadMissingDir(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("want error")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	task := Task{ID: "kanban", Title: "Kanban", Category: "ui", Type: "review",
		TimeoutMinutes: 45, Prompt: "Build a kanban board.\nWith drag and drop.\n"}
	if err := Save(dir, task); err != nil {
		t.Fatal(err)
	}
	lib, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Tasks) != 1 || len(lib.Warnings) != 0 {
		t.Fatalf("tasks=%d warnings=%v", len(lib.Tasks), lib.Warnings)
	}
	if lib.Tasks[0] != task {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", lib.Tasks[0], task)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	bad := []Task{
		{ID: "Bad ID", Title: "x", Type: "review", Prompt: "p"},
		{ID: "ok", Title: "x", Type: "check", Prompt: "p"},               // check without script
		{ID: "ok2", Title: "x", Type: "review", Prompt: "p", Check: "c"}, // review with script
		{ID: "ok3", Title: "x", Type: "review"},                          // no prompt
	}
	for i, task := range bad {
		if err := Save(dir, task); err == nil {
			t.Fatalf("case %d: want validation error for %+v", i, task)
		}
	}
}
