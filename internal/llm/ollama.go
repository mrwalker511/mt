package llm

import "context"

// Config holds the LLM provider settings, parsed from the top-level llm: YAML key.
type Config struct {
	Provider   string `yaml:"provider"`    // "litellm" (default) | "llamacpp" | "openai" | "apple"
	Model      string `yaml:"model"`       // model name as understood by the chosen provider
	BaseURL    string `yaml:"base_url"`    // override the provider's default endpoint
	APIKey     string `yaml:"api_key"`     // OpenAI only; falls back to OPENAI_API_KEY env var
	BridgePath string `yaml:"bridge_path"` // apple only: path to mt-apple-bridge binary
}

// Generate dispatches to the configured provider and returns the full response text.
// Returns a human-readable error the TUI can display directly.
func Generate(ctx context.Context, cfg Config, prompt string) (string, error) {
	switch cfg.Provider {
	case "openai":
		return generateOpenAI(ctx, cfg, prompt)
	case "llamacpp":
		return generateLlamaCpp(ctx, cfg, prompt)
	case "apple":
		return generateApple(ctx, cfg, prompt)
	default: // "litellm" or ""
		return generateLiteLLM(ctx, cfg, prompt)
	}
}
