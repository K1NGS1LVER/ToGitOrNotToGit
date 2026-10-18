package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
)

// ModelsURLFor derives a provider's model-listing endpoint from its
// chat-completions base URL (e.g. ".../v1/chat/completions" -> ".../v1/models").
func ModelsURLFor(cfg config.Config) string {
	base := strings.TrimRight(baseURLFor(cfg), "/")
	base = strings.TrimSuffix(base, "/chat/completions")
	return base + "/models"
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// ListModels returns the sorted model IDs the provider reports for cfg's key.
// The Authorization header is omitted when no key is configured so local
// providers such as Ollama work without one.
func ListModels(ctx context.Context, cfg config.Config) ([]string, error) {
	provider := config.ProviderLabel(cfg.Provider)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ModelsURLFor(cfg), nil)
	if err != nil {
		return nil, &Error{Kind: KindUnknown, Provider: provider, Message: "building models request", Err: err}
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, &Error{Kind: KindNetwork, Provider: provider, Message: "requesting models", Err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		code, message := parseProviderError(raw)
		return nil, &Error{
			Kind:     classify(resp.StatusCode, code, message),
			Status:   resp.StatusCode,
			Provider: provider,
			Message:  message,
		}
	}

	var parsed modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, &Error{Kind: KindInvalidOutput, Provider: provider, Message: "decoding models response", Err: err}
	}

	ids := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}
