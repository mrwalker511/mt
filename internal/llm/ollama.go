package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// DefaultBaseURL is the Ollama API endpoint used when none is configured.
const DefaultBaseURL = "http://localhost:11434"

// DefaultModel is the Ollama model used when none is configured.
const DefaultModel = "llama3.2"

// Config holds the LLM provider settings, parsed from the top-level llm: YAML key.
type Config struct {
	Provider string `yaml:"provider"` // only "ollama" is supported today
	Model    string `yaml:"model"`    // e.g. "llama3.2", "mistral", "llama3"
	BaseURL  string `yaml:"base_url"` // default: http://localhost:11434
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

// Generate sends prompt to Ollama (non-streaming) and returns the full response text.
// Returns a human-readable error if Ollama is not reachable so the TUI can display it.
func Generate(ctx context.Context, cfg Config, prompt string) (string, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	body, err := json.Marshal(generateRequest{Model: model, Prompt: prompt, Stream: false})
	if err != nil {
		return "", fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama not reachable — start with: ollama serve (%w)", err)
	}
	defer resp.Body.Close()

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("ollama: %s", result.Error)
	}
	return strings.TrimSpace(result.Response), nil
}
