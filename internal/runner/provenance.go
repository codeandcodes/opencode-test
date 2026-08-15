package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"opencode-bench/internal/tasks"
)

// Provenance snapshots everything a run's results depend on, so cells stay
// attributable after task prompts or model configs change.
type Provenance struct {
	Model          string     `json:"model"`
	PromptSHA      string     `json:"prompt_sha"`
	Task           tasks.Task `json:"task"`
	LlamaSwapEntry any        `json:"llama_swap_entry,omitempty"`
	CapturedAt     time.Time  `json:"captured_at"`
}

// PromptSHA returns the hex sha256 of a task prompt.
func PromptSHA(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

// buildProvenance captures the run's inputs. llamaSwapConfig may be empty;
// entry capture is best-effort and never fails the run.
func buildProvenance(task tasks.Task, model, llamaSwapConfig string) Provenance {
	p := Provenance{
		Model:      model,
		PromptSHA:  PromptSHA(task.Prompt),
		Task:       task,
		CapturedAt: time.Now().UTC(),
	}
	if llamaSwapConfig != "" {
		if raw, err := os.ReadFile(llamaSwapConfig); err == nil {
			var doc struct {
				Models map[string]any `yaml:"models"`
			}
			if yaml.Unmarshal(raw, &doc) == nil {
				if entry, ok := doc.Models[model]; ok {
					p.LlamaSwapEntry = entry
				}
			}
		}
	}
	return p
}

func writeProvenance(path string, p Provenance) error {
	out, err := json.MarshalIndent(p, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
