package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDiscoverModels(t *testing.T) {
	p := writeTemp(t, `{"provider":{"llama-swap":{"models":{
		"b-model":{"name":"B Model"},"a-model":{"name":"A Model"}}}}}`)
	models, err := DiscoverModels(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	if models[0].ID != "a-model" || models[0].Name != "A Model" {
		t.Fatalf("models not sorted by ID or name lost: %+v", models)
	}
	if models[1].ID != "b-model" {
		t.Fatalf("second model = %+v", models[1])
	}
}

func TestDiscoverModelsNoProvider(t *testing.T) {
	p := writeTemp(t, `{"provider":{"google":{"models":{}}}}`)
	_, err := DiscoverModels(p)
	if err == nil || !strings.Contains(err.Error(), "llama-swap") {
		t.Fatalf("err = %v, want mention of llama-swap", err)
	}
}

func TestDiscoverModelsMissingFile(t *testing.T) {
	if _, err := DiscoverModels(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestDiscoverModelsBadJSON(t *testing.T) {
	if _, err := DiscoverModels(writeTemp(t, `{not json`)); err == nil {
		t.Fatal("want error for bad JSON")
	}
}
