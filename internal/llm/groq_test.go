package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGroqClient_Generate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header = %q, want Bearer test-key", r.Header.Get("Authorization"))
		}
		resp := chatResponse{}
		resp.Choices = []struct {
			Message chatMessage `json:"message"`
		}{{Message: chatMessage{Role: "assistant", Content: "feat(auth): a tale of two tokens"}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGroqClient("test-key", "test-model")
	client.BaseURL = server.URL

	got, err := client.Generate(context.Background(), Request{Persona: "soap-opera", Stats: "1 file", Diff: "+x"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if got != "feat(auth): a tale of two tokens" {
		t.Errorf("Generate() = %q, want the canned message", got)
	}
}

func TestGroqClient_Generate_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewGroqClient("test-key", "test-model")
	client.BaseURL = server.URL

	_, err := client.Generate(context.Background(), Request{})
	if err == nil {
		t.Fatal("Generate returned nil error, want error for 500 response")
	}
}

func TestGroqClient_Generate_MissingAPIKey(t *testing.T) {
	client := NewGroqClient("", "test-model")
	_, err := client.Generate(context.Background(), Request{})
	if err == nil {
		t.Fatal("Generate returned nil error, want error for missing API key")
	}
}

func TestGroqClient_Generate_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	client := NewGroqClient("test-key", "test-model")
	client.BaseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	_, err := client.Generate(ctx, Request{})
	if err == nil {
		t.Fatal("Generate returned nil error, want timeout error")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("error = %v, want context deadline exceeded", err)
	}
}
