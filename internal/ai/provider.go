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

// NewProvider creates a Provider based on the provider name.
// Defaults to GitHub Copilot SDK when provider is "" or "copilot".
func NewProvider(provider, token, model, baseURL, cwd string) Provider {
	switch provider {
	case "openai":
		return NewOpenAIProvider(token, model, baseURL)
	case "anthropic":
		return NewAnthropicProvider(token, model)
	default: // "copilot" or empty
		return NewSDKProvider(token, model, cwd)
	}
}
