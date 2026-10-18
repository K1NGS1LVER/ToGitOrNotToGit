package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		code    string
		message string
		want    ErrorKind
	}{
		{"unauthorized", 401, "", "invalid api key", KindAuth},
		{"forbidden", 403, "", "no access", KindAuth},
		{"not found", 404, "", "not found", KindModelUnavailable},
		{"model_not_found code", 400, "model_not_found", "The model does not exist", KindModelUnavailable},
		{"model_decommissioned code", 400, "model_decommissioned", "retired", KindModelUnavailable},
		{"does not exist message", 400, "", "The model `x` does not exist", KindModelUnavailable},
		{"rate limited", 429, "", "slow down", KindRateLimit},
		{"server error", 500, "", "boom", KindServer},
		{"bad gateway", 502, "", "bad gateway", KindServer},
		{"unclassified", 400, "", "bad request", KindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.status, tt.code, tt.message); got != tt.want {
				t.Errorf("classify(%d, %q, %q) = %v, want %v", tt.status, tt.code, tt.message, got, tt.want)
			}
		})
	}
}

func TestParseProviderError(t *testing.T) {
	code, message := parseProviderError([]byte(`{"error":{"message":"The model does not exist","type":"invalid_request_error","code":"model_not_found"}}`))
	if code != "model_not_found" {
		t.Errorf("code = %q, want model_not_found", code)
	}
	if message != "The model does not exist" {
		t.Errorf("message = %q, want the provider message", message)
	}

	code, message = parseProviderError([]byte("plain text failure"))
	if code != "" {
		t.Errorf("code = %q, want empty for non-JSON body", code)
	}
	if message != "plain text failure" {
		t.Errorf("message = %q, want the raw body", message)
	}
}

func TestKindOf(t *testing.T) {
	if KindOf(nil) != KindUnknown {
		t.Error("KindOf(nil) should be KindUnknown")
	}
	if KindOf(errors.New("plain")) != KindUnknown {
		t.Error("KindOf(plain error) should be KindUnknown")
	}
	wrapped := &Error{Kind: KindModelUnavailable, Model: "m"}
	if !IsModelUnavailable(wrapped) {
		t.Error("IsModelUnavailable should be true for a KindModelUnavailable error")
	}
	if KindOf(errors.Join(errors.New("x"), wrapped)) != KindModelUnavailable {
		t.Error("KindOf should unwrap joined errors")
	}
}

func TestGenerate_ClassifiesStatuses(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantKind ErrorKind
	}{
		{"retired model", 404, `{"error":{"message":"The model does not exist","code":"model_not_found"}}`, KindModelUnavailable},
		{"bad key", 401, `{"error":{"message":"invalid api key"}}`, KindAuth},
		{"rate limited", 429, `{"error":{"message":"too many requests"}}`, KindRateLimit},
		{"server error", 500, `{"error":{"message":"internal"}}`, KindServer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewGroqClient("k", "m")
			client.BaseURL = server.URL

			_, err := client.Generate(context.Background(), Request{})
			if got := KindOf(err); got != tt.wantKind {
				t.Fatalf("KindOf(err) = %v, want %v (err: %v)", got, tt.wantKind, err)
			}
		})
	}
}

func TestGenerate_MissingKeyIsAuth(t *testing.T) {
	client := NewGroqClient("", "m")
	_, err := client.Generate(context.Background(), Request{})
	if KindOf(err) != KindAuth {
		t.Fatalf("KindOf(err) = %v, want KindAuth", KindOf(err))
	}
}

func TestGenerate_InvalidOutputIsClassified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{}
		resp.Choices = []struct {
			Message chatMessage `json:"message"`
		}{{Message: chatMessage{Role: "assistant", Content: "not a conventional commit"}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGroqClient("k", "m")
	client.BaseURL = server.URL

	_, err := client.Generate(context.Background(), Request{})
	if KindOf(err) != KindInvalidOutput {
		t.Fatalf("KindOf(err) = %v, want KindInvalidOutput", KindOf(err))
	}
}
