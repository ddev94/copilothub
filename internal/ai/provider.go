package ai

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Provider is the interface for AI completion backends.
type Provider interface {
	Complete(ctx context.Context, messages []Message) (string, error)
}
