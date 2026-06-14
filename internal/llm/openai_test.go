package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateOpenAI_HTTP500_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer srv.Close()

	cfg := Config{Provider: "openai", BaseURL: srv.URL, APIKey: "test-key"}
	_, err := Generate(context.Background(), cfg, "system", "hello")
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected status code in error, got: %v", err)
	}
}

func TestGenerateOpenAI_LargeBody_NotOOM(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write 2 MB of valid-ish JSON prefix to test the size cap.
		// The decoder will fail or truncate — either way no OOM.
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"`))
		for i := 0; i < 2*1024*1024; i++ {
			w.Write([]byte("x"))
		}
		// intentionally no closing — the LimitReader cuts it off cleanly
	}))
	defer srv.Close()

	cfg := Config{Provider: "litellm", BaseURL: srv.URL}
	// expect either a decode error (truncated JSON) or a valid response — not a hang or OOM
	_, _ = Generate(context.Background(), cfg, "system", "hello")
}

func TestGenerateOpenAI_BadBaseURL_ReturnsError(t *testing.T) {
	cfg := Config{Provider: "litellm", BaseURL: "not a valid url"}
	_, err := Generate(context.Background(), cfg, "system", "hello")
	if err == nil {
		t.Fatal("expected error for invalid base_url")
	}
	if !strings.Contains(err.Error(), "invalid base_url") {
		t.Errorf("expected invalid base_url in error, got: %v", err)
	}
}

func TestGenerateOpenAI_MissingKey_ReturnsError(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	cfg := Config{Provider: "openai", Model: "gpt-4o-mini"}
	_, err := Generate(context.Background(), cfg, "system", "hello")
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
		var req openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		if len(req.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(req.Messages))
		} else {
			if req.Messages[0].Role != "system" {
				t.Errorf("expected messages[0].role=system, got %q", req.Messages[0].Role)
			}
			if req.Messages[1].Role != "user" {
				t.Errorf("expected messages[1].role=user, got %q", req.Messages[1].Role)
			}
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
	got, err := Generate(context.Background(), cfg, "sys prompt", "test query")
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
		var req openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		if len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
			t.Errorf("expected system+user messages, got %+v", req.Messages)
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
		got, err := Generate(context.Background(), cfg, "sys", "test")
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
		var req openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		if len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
			t.Errorf("expected system+user messages, got %+v", req.Messages)
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
	got, err := Generate(context.Background(), cfg, "sys", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "INFO: llamacpp reply" {
		t.Errorf("unexpected response: %q", got)
	}
}
