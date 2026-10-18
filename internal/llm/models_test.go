package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
)

func TestModelsURLFor(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{"groq default", config.Config{Provider: "groq"}, "https://api.groq.com/openai/v1/models"},
		{"openai default", config.Config{Provider: "openai"}, "https://api.openai.com/v1/models"},
		{"ollama default", config.Config{Provider: "ollama"}, "http://localhost:11434/v1/models"},
		{
			"custom chat base url is rewritten",
			config.Config{Provider: "openai-compatible", BaseURL: "https://example.test/v1/chat/completions"},
			"https://example.test/v1/models",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ModelsURLFor(tt.cfg); got != tt.want {
				t.Errorf("ModelsURLFor(%+v) = %q, want %q", tt.cfg, got, tt.want)
			}
		})
	}
}

func TestListModels_SortsAndSendsAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/models" {
			t.Errorf("path = %q, want /models", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"zeta"},{"id":"alpha"},{"id":""}]}`))
	}))
	defer server.Close()

	cfg := config.Config{Provider: "groq", BaseURL: server.URL, APIKey: "k"}
	ids, err := ListModels(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ListModels returned error: %v", err)
	}
	if gotAuth != "Bearer k" {
		t.Errorf("Authorization = %q, want Bearer k", gotAuth)
	}
	if want := []string{"alpha", "zeta"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("ListModels() = %v, want %v", ids, want)
	}
}

func TestListModels_OmitsAuthWithoutKey(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	cfg := config.Config{Provider: "ollama", BaseURL: server.URL}
	if _, err := ListModels(context.Background(), cfg); err != nil {
		t.Fatalf("ListModels returned error: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty when no key is configured", gotAuth)
	}
}

func TestListModels_NonOKIsClassified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	cfg := config.Config{Provider: "groq", BaseURL: server.URL, APIKey: "k"}
	_, err := ListModels(context.Background(), cfg)
	if KindOf(err) != KindAuth {
		t.Fatalf("KindOf(err) = %v, want KindAuth (err: %v)", KindOf(err), err)
	}
}
