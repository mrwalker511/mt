package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateOpenAI_MissingKey_ReturnsError(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	cfg := Config{Provider: "openai", Model: "gpt-4o-mini"}
	_, err := Generate(context.Background(), cfg, "hello")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "no API key") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerateOpenAI_Dispatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		resp := openAIResponse{
			Choices: []openAIChoice{
				{Message: openAIMessage{Role: "assistant", Content: "INFO: test answer"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := Config{Provider: "openai", Model: "gpt-4o-mini", BaseURL: srv.URL, APIKey: "test-key"}
	got, err := Generate(context.Background(), cfg, "test query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "INFO: test answer" {
		t.Errorf("unexpected response: %q", got)
	}
}

func TestGenerateLiteLLM_IsDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("litellm should not send auth header by default, got: %s", auth)
		}
		resp := openAIResponse{
			Choices: []openAIChoice{
				{Message: openAIMessage{Role: "assistant", Content: "INFO: litellm reply"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	for _, provider := range []string{"", "litellm"} {
		cfg := Config{Provider: provider, Model: "llama3.1:8b", BaseURL: srv.URL}
		got, err := Generate(context.Background(), cfg, "test")
		if err != nil {
			t.Fatalf("provider=%q unexpected error: %v", provider, err)
		}
		if got != "INFO: litellm reply" {
			t.Errorf("provider=%q unexpected response: %q", provider, got)
		}
	}
}

func TestGenerateLlamaCpp_Dispatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := openAIResponse{
			Choices: []openAIChoice{
				{Message: openAIMessage{Role: "assistant", Content: "INFO: llamacpp reply"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := Config{Provider: "llamacpp", BaseURL: srv.URL}
	got, err := Generate(context.Background(), cfg, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "INFO: llamacpp reply" {
		t.Errorf("unexpected response: %q", got)
	}
}
