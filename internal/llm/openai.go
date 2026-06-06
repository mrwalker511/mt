package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const (
	DefaultOpenAIBaseURL   = "https://api.openai.com"
	DefaultOpenAIModel     = "gpt-4o-mini"
	DefaultLiteLLMBaseURL  = "http://localhost:4000"
	DefaultLlamaCppBaseURL = "http://localhost:8080"
)

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
	Error   *openAIError   `json:"error,omitempty"`
}

type openAIError struct {
	Message string `json:"message"`
}

// generateOpenAICompat posts to any OpenAI-compatible /v1/chat/completions endpoint.
// apiKey is sent as a Bearer token only when non-empty.
func generateOpenAICompat(ctx context.Context, baseURL, model, apiKey, prompt string) (string, error) {
	body, err := json.Marshal(openAIRequest{
		Model:    model,
		Messages: []openAIMessage{{Role: "user", Content: prompt}},
		Stream:   false,
	})
	if err != nil {
		return "", fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("not reachable at %s — is the server running? (%w)", baseURL, err)
	}
	defer resp.Body.Close()

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("server error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func generateOpenAI(ctx context.Context, cfg Config, prompt string) (string, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("openai: no API key — set api_key in config or OPENAI_API_KEY env var")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultOpenAIBaseURL
	}
	model := cfg.Model
	if model == "" {
		model = DefaultOpenAIModel
	}
	result, err := generateOpenAICompat(ctx, baseURL, model, apiKey, prompt)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	return result, nil
}

func generateLiteLLM(ctx context.Context, cfg Config, prompt string) (string, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultLiteLLMBaseURL
	}
	result, err := generateOpenAICompat(ctx, baseURL, cfg.Model, cfg.APIKey, prompt)
	if err != nil {
		return "", fmt.Errorf("litellm: %w", err)
	}
	return result, nil
}

func generateLlamaCpp(ctx context.Context, cfg Config, prompt string) (string, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultLlamaCppBaseURL
	}
	result, err := generateOpenAICompat(ctx, baseURL, cfg.Model, cfg.APIKey, prompt)
	if err != nil {
		return "", fmt.Errorf("llamacpp: %w", err)
	}
	return result, nil
}
