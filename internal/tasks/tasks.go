// Package tasks loads and validates the benchmark task library.
package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultTimeoutMinutes = 30

var idRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// Task is one benchmark task definition.
type Task struct {
	ID             string `yaml:"id" json:"id"`
	Title          string `yaml:"title" json:"title"`
	Category       string `yaml:"category" json:"category"`
	Type           string `yaml:"type" json:"type"`
	TimeoutMinutes int    `yaml:"timeout_minutes" json:"timeout_minutes"`
	Prompt         string `yaml:"prompt" json:"prompt"`
	Check          string `yaml:"check,omitempty" json:"check,omitempty"`
}

// Library is the loaded task set plus warnings for files that didn't load.
type Library struct {
	Tasks    []Task   `json:"tasks"`
	Warnings []string `json:"warnings"`
}

// Get returns the task with the given id.
func (l Library) Get(id string) (Task, bool) {
	for _, t := range l.Tasks {
		if t.ID == id {
			return t, true
		}
	}
	return Task{}, false
}

func validate(t Task) error {
	if !idRe.MatchString(t.ID) {
		return fmt.Errorf("id %q must match %s", t.ID, idRe)
	}
	if t.Type != "review" && t.Type != "check" {
		return fmt.Errorf("type %q must be review or check", t.Type)
	}
	if strings.TrimSpace(t.Prompt) == "" {
		return fmt.Errorf("prompt is empty")
	}
	hasCheck := strings.TrimSpace(t.Check) != ""
	if t.Type == "check" && !hasCheck {
		return fmt.Errorf("check task has no check script")
	}
	if t.Type == "review" && hasCheck {
		return fmt.Errorf("review task must not have a check script")
	}
	return nil
}

// Load reads every *.yaml in dir. Invalid files become Warnings, not errors.
func Load(dir string) (Library, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Library{}, fmt.Errorf("read tasks dir: %w", err)
	}
	lib := Library{Warnings: []string{}}
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			lib.Warnings = append(lib.Warnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		var t Task
		if err := yaml.Unmarshal(raw, &t); err != nil {
			lib.Warnings = append(lib.Warnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if t.TimeoutMinutes == 0 {
			t.TimeoutMinutes = defaultTimeoutMinutes
		}
		if err := validate(t); err != nil {
			lib.Warnings = append(lib.Warnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if stem := strings.TrimSuffix(name, ".yaml"); stem != t.ID {
			lib.Warnings = append(lib.Warnings, fmt.Sprintf("%s: id %q does not match filename", name, t.ID))
			continue
		}
		if seen[t.ID] {
			lib.Warnings = append(lib.Warnings, fmt.Sprintf("%s: duplicate id %q", name, t.ID))
			continue
		}
		seen[t.ID] = true
		lib.Tasks = append(lib.Tasks, t)
	}
	sort.Slice(lib.Tasks, func(i, j int) bool {
		a, b := lib.Tasks[i], lib.Tasks[j]
		if a.Type != b.Type {
			return a.Type == "review" // reviews first
		}
		return a.ID < b.ID
	})
	return lib, nil
}

// Save validates t and writes it as tasks/<id>.yaml.
func Save(dir string, t Task) error {
	if t.TimeoutMinutes == 0 {
		t.TimeoutMinutes = defaultTimeoutMinutes
	}
	if err := validate(t); err != nil {
		return err
	}
	out, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, t.ID+".yaml"), out, 0o644)
}
