package ai

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolEvent represents a single tool call the model made during completion.
type ToolEvent struct {
	Kind string `json:"kind"` // "read", "write", "shell", "url", "mcp", "custom-tool"
	Path string `json:"path,omitempty"`
	Name string `json:"name,omitempty"`
}

// Tool is a custom function the AI can invoke during a completion session.
// Handler receives JSON-decoded arguments and returns a text result for the LLM.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Handler     func(args map[string]any) (string, error)
}

// Provider is the interface for AI completion backends.
type Provider interface {
	Complete(ctx context.Context, messages []Message) (string, error)
}

// EventingProvider is an optional extension of Provider that streams tool events
// as they happen during completion. Only SDKProvider implements this.
type EventingProvider interface {
	CompleteWithEvents(ctx context.Context, messages []Message, tools []Tool, onEvent func(ToolEvent)) (string, error)
}

// ChatSessionProvider extends Provider with persistent multi-turn session support.
// After CompleteWithSession the session is disconnected but state is preserved on disk,
// allowing ChatWithSession to resume the same conversation later.
// Only SDKProvider implements this.
type ChatSessionProvider interface {
	CompleteWithSession(ctx context.Context, messages []Message, tools []Tool, onEvent func(ToolEvent)) (result, sessionID string, err error)
	ChatWithSession(ctx context.Context, sessionID, message string, tools []Tool, onEvent func(ToolEvent)) (string, error)
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
