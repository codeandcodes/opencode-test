package tasks

import "testing"

// TestRealTaskLibrary guards the shipped tasks/ directory: every file must
// parse cleanly and the full set must be present.
func TestRealTaskLibrary(t *testing.T) {
	lib, err := Load("../../tasks")
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Warnings) != 0 {
		t.Fatalf("task library has warnings: %v", lib.Warnings)
	}
	if len(lib.Tasks) != 15 {
		ids := make([]string, 0, len(lib.Tasks))
		for _, task := range lib.Tasks {
			ids = append(ids, task.ID)
		}
		t.Fatalf("got %d tasks, want 15: %v", len(lib.Tasks), ids)
	}
	reviews, checks := 0, 0
	for _, task := range lib.Tasks {
		switch task.Type {
		case "review":
			reviews++
		case "check":
			checks++
		}
	}
	if reviews != 10 || checks != 5 {
		t.Fatalf("reviews=%d checks=%d, want 10/5", reviews, checks)
	}
}
