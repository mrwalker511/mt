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

func TestGenerateOpenAI_Dispatch_Provider(t *testing.T) {
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

	cfg := Config{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		BaseURL:  srv.URL,
		APIKey:   "test-key",
	}
	got, err := Generate(context.Background(), cfg, "test query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "INFO: test answer" {
		t.Errorf("unexpected response: %q", got)
	}
}

func TestGenerateOllama_StillDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := generateResponse{Response: "INFO: ollama reply", Done: true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	for _, provider := range []string{"", "ollama"} {
		cfg := Config{Provider: provider, Model: "llama3.2", BaseURL: srv.URL}
		got, err := Generate(context.Background(), cfg, "test")
		if err != nil {
			t.Fatalf("provider=%q unexpected error: %v", provider, err)
		}
		if got != "INFO: ollama reply" {
			t.Errorf("provider=%q unexpected response: %q", provider, got)
		}
	}
}
