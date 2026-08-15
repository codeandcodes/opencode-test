// Package config discovers local models from opencode's configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Model is one entry of the llama-swap provider in opencode.json.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DiscoverModels reads opencode.json and returns the llama-swap provider's
// models sorted by ID. The file is read on every call so config edits are
// picked up without a restart.
func DiscoverModels(path string) ([]Model, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read opencode config: %w", err)
	}
	var doc struct {
		Provider map[string]struct {
			Models map[string]struct {
				Name string `json:"name"`
			} `json:"models"`
		} `json:"provider"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse opencode config: %w", err)
	}
	prov, ok := doc.Provider["llama-swap"]
	if !ok {
		return nil, fmt.Errorf("no llama-swap provider in %s", path)
	}
	models := make([]Model, 0, len(prov.Models))
	for id, m := range prov.Models {
		name := m.Name
		if name == "" {
			name = id
		}
		models = append(models, Model{ID: id, Name: name})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}
