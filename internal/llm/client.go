package llm

import "context"

type Request struct {
	Persona string
	Stats   string
	Diff    string
}

type Client interface {
	Generate(ctx context.Context, req Request) (string, error)
}
