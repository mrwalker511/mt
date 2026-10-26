// Package llm sends a single system+user prompt to a configured language model
// and returns the reply text. Every provider except "apple" speaks the
// OpenAI-compatible /v1/chat/completions API; "apple" shells out to the
// mt-apple-bridge binary, which wraps Apple's on-device Foundation Models.
package llm

import (
	"context"
	"fmt"
	"os"
)

// Config holds the LLM provider settings, parsed from the top-level llm: YAML key.
type Config struct {
	Provider   string `yaml:"provider"`    // "litellm" (default) | "llamacpp" | "mlx" | "openai" | "apple"
	Model      string `yaml:"model"`       // model name as understood by the chosen provider
	BaseURL    string `yaml:"base_url"`    // override the provider's default endpoint (without /v1)
	APIKey     string `yaml:"api_key"`     // sent as a Bearer token; openai falls back to OPENAI_API_KEY
	BridgePath string `yaml:"bridge_path"` // apple only: path to mt-apple-bridge binary
	Debug      bool   `yaml:"debug"`       // append request/response to ~/.mt/logs/debug.log
}

const (
	DefaultOpenAIBaseURL   = "https://api.openai.com"
	DefaultOpenAIModel     = "gpt-4o-mini"
	DefaultLiteLLMBaseURL  = "http://localhost:4000"
	DefaultLlamaCppBaseURL = "http://localhost:8080"
	DefaultMLXBaseURL      = "http://localhost:8000"
)

// defaultBaseURLs lists the OpenAI-compatible providers and their default endpoints.
var defaultBaseURLs = map[string]string{
	"litellm":  DefaultLiteLLMBaseURL,
	"llamacpp": DefaultLlamaCppBaseURL,
	"mlx":      DefaultMLXBaseURL,
	"openai":   DefaultOpenAIBaseURL,
}

// Generate dispatches to the configured provider and returns the full response text.
// Errors are prefixed with the provider name and are suitable for direct display.
func Generate(ctx context.Context, cfg Config, systemPrompt, userMessage string) (string, error) {
	provider := cfg.Provider
	if provider == "" {
		provider = "litellm"
	}
	if provider == "apple" {
		return generateApple(ctx, cfg, systemPrompt, userMessage)
	}
	baseURL, ok := defaultBaseURLs[provider]
	if !ok {
		return "", fmt.Errorf("unknown llm provider %q (use litellm, llamacpp, mlx, openai or apple)", provider)
	}
	if cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}
	model, apiKey := cfg.Model, cfg.APIKey
	if provider == "openai" {
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return "", fmt.Errorf("openai: no API key — set api_key in config or OPENAI_API_KEY env var")
		}
		if model == "" {
			model = DefaultOpenAIModel
		}
	}
	result, err := generateOpenAICompat(ctx, baseURL, model, apiKey, systemPrompt, userMessage)
	if err != nil {
		return "", fmt.Errorf("%s: %w", provider, err)
	}
	return result, nil
}
