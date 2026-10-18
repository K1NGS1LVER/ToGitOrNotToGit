package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
)

func chatOK(content string) string {
	resp := chatResponse{}
	resp.Choices = []struct {
		Message chatMessage `json:"message"`
	}{{Message: chatMessage{Role: "assistant", Content: content}}}
	b, _ := json.Marshal(resp)
	return string(b)
}

// modelServer responds based on the model named in the request body, recording
// every model it is asked for.
func modelServer(t *testing.T, calls *[]string, reply func(model string) (int, string)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		*calls = append(*calls, req.Model)
		status, body := reply(req.Model)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestFallbackClient_CascadesOnUnavailableModel(t *testing.T) {
	var calls []string
	server := modelServer(t, &calls, func(model string) (int, string) {
		if model == "primary" {
			return http.StatusNotFound, `{"error":{"message":"The model does not exist","code":"model_not_found"}}`
		}
		return http.StatusOK, chatOK("feat: from the fallback")
	})
	defer server.Close()

	cfg := config.Config{
		Provider:       "groq",
		Model:          "primary",
		FallbackModels: []string{"secondary"},
		APIKey:         "k",
		BaseURL:        server.URL,
		TimeoutMS:      2500,
	}

	msg, err := New(cfg).Generate(context.Background(), Request{})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if msg != "feat: from the fallback" {
		t.Errorf("message = %q, want the fallback model's message", msg)
	}
	if len(calls) != 2 || calls[0] != "primary" || calls[1] != "secondary" {
		t.Errorf("calls = %v, want [primary secondary]", calls)
	}
}

func TestFallbackClient_StopsOnNonModelFailure(t *testing.T) {
	var calls []string
	server := modelServer(t, &calls, func(string) (int, string) {
		return http.StatusUnauthorized, `{"error":{"message":"invalid api key"}}`
	})
	defer server.Close()

	cfg := config.Config{
		Provider:       "groq",
		Model:          "primary",
		FallbackModels: []string{"secondary"},
		APIKey:         "k",
		BaseURL:        server.URL,
		TimeoutMS:      2500,
	}

	_, err := New(cfg).Generate(context.Background(), Request{})
	if KindOf(err) != KindAuth {
		t.Fatalf("KindOf(err) = %v, want KindAuth", KindOf(err))
	}
	if len(calls) != 1 {
		t.Errorf("calls = %v, want a single attempt (auth cannot be fixed by another model)", calls)
	}
}

func TestFallbackClient_AllUnavailableReturnsLastError(t *testing.T) {
	var calls []string
	server := modelServer(t, &calls, func(string) (int, string) {
		return http.StatusNotFound, `{"error":{"message":"The model does not exist","code":"model_not_found"}}`
	})
	defer server.Close()

	cfg := config.Config{
		Provider:       "groq",
		Model:          "a",
		FallbackModels: []string{"b", "c"},
		APIKey:         "k",
		BaseURL:        server.URL,
		TimeoutMS:      2500,
	}

	_, err := New(cfg).Generate(context.Background(), Request{})
	if !IsModelUnavailable(err) {
		t.Fatalf("IsModelUnavailable(err) = false, want true (err: %v)", err)
	}
	if len(calls) != 3 {
		t.Errorf("calls = %v, want all three models tried", calls)
	}
}

func TestFallbackClient_DedupesRepeatedModels(t *testing.T) {
	var calls []string
	server := modelServer(t, &calls, func(model string) (int, string) {
		if model == "primary" {
			return http.StatusNotFound, `{"error":{"code":"model_not_found","message":"does not exist"}}`
		}
		return http.StatusOK, chatOK("feat: ok")
	})
	defer server.Close()

	cfg := config.Config{
		Provider:       "groq",
		Model:          "primary",
		FallbackModels: []string{"primary", "secondary"},
		APIKey:         "k",
		BaseURL:        server.URL,
		TimeoutMS:      2500,
	}

	if got := cfg.Models(); len(got) != 2 {
		t.Fatalf("cfg.Models() = %v, want 2 de-duplicated entries", got)
	}

	if _, err := New(cfg).Generate(context.Background(), Request{}); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if len(calls) != 2 || calls[0] != "primary" || calls[1] != "secondary" {
		t.Errorf("calls = %v, want [primary secondary]", calls)
	}
}

func TestNew_SingleModelReturnsPlainClient(t *testing.T) {
	client := New(config.Config{Provider: "openai", Model: "gpt-x", APIKey: "k"})
	if _, ok := client.(*OpenAIClient); !ok {
		t.Fatalf("New() with no fallbacks returned %T, want *OpenAIClient", client)
	}
}
