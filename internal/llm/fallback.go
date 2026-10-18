package llm

import (
	"context"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/logx"
)

// fallbackClient tries an ordered list of models (primary first) on a single
// provider, advancing to the next model only when the current one is reported
// unavailable. Auth, timeout, network, and rate-limit failures fail fast - a
// different model on the same provider cannot fix them, and retrying would only
// slow the hook down.
type fallbackClient struct {
	models []string
	build  func(model string) Client
}

func (c *fallbackClient) Generate(ctx context.Context, req Request) (string, error) {
	var lastErr error
	for i, model := range c.models {
		message, err := c.build(model).Generate(ctx, req)
		if err == nil {
			if i > 0 {
				logx.Debugf("primary model %q unavailable; used fallback model %q", c.models[0], model)
			}
			return message, nil
		}

		lastErr = err
		if !IsModelUnavailable(err) {
			return "", err
		}
		if i < len(c.models)-1 {
			logx.Debugf("model %q unavailable (%v); trying %q", model, err, c.models[i+1])
		}
	}
	return "", lastErr
}
