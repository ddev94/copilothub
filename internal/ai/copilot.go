package ai

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	copilot "github.com/github/copilot-sdk/go"
)

// FindCLI returns the path to the GitHub Copilot CLI binary.
// Checks COPILOT_CLI_PATH env → PATH → known VS Code extension locations.
func FindCLI() string {
	if path := os.Getenv("COPILOT_CLI_PATH"); path != "" {
		return path
	}
	if path, err := exec.LookPath("copilot"); err == nil {
		return path
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		// macOS – VS Code
		filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage", "github.copilot-chat", "copilotCli", "copilot"),
		// Linux – VS Code
		filepath.Join(home, ".config", "Code", "User", "globalStorage", "github.copilot-chat", "copilotCli", "copilot"),
		// Windows – VS Code
		filepath.Join(os.Getenv("APPDATA"), "Code", "User", "globalStorage", "github.copilot-chat", "copilotCli", "copilot"),
		// macOS – VS Code Insiders
		filepath.Join(home, "Library", "Application Support", "Code - Insiders", "User", "globalStorage", "github.copilot-chat", "copilotCli", "copilot"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// SDKProvider uses the GitHub Copilot SDK for AI completions.
// Authentication is handled automatically via the gh CLI logged-in user.
// An optional token override can be provided in ClientOptions.
type SDKProvider struct {
	once     sync.Once
	startErr error
	client   *copilot.Client
	model    string
	token    string // optional GitHub token override; uses gh auth if empty
	cwd      string // working directory / repo path
}

func NewSDKProvider(token, model, cwd string) *SDKProvider {
	return &SDKProvider{token: token, model: "gpt-4.1", cwd: cwd}
}

func (p *SDKProvider) start() {
	opts := &copilot.ClientOptions{LogLevel: "error"}
	if cli := FindCLI(); cli != "" {
		opts.CLIPath = cli
	}
	if p.token != "" {
		opts.GitHubToken = p.token
	}
	if p.cwd != "" {
		opts.Cwd = p.cwd
	}
	// UseLoggedInUser defaults to true → uses gh CLI auth automatically
	p.client = copilot.NewClient(opts)
	p.startErr = p.client.Start(context.Background())
}

func (p *SDKProvider) Stop() {
	p.once.Do(func() {}) // prevent double-start if Stop called before any request
	if p.client != nil {
		p.client.Stop()
	}
}

func (p *SDKProvider) Complete(ctx context.Context, messages []Message) (string, error) {
	return p.CompleteWithEvents(ctx, messages, nil, nil)
}

func convertTools(tools []Tool, onEvent func(ToolEvent)) []copilot.Tool {
	result := make([]copilot.Tool, len(tools))
	for i, t := range tools {
		result[i] = copilot.Tool{
			Name:           t.Name,
			Description:    t.Description,
			Parameters:     t.Parameters,
			SkipPermission: true,
			Handler: func(inv copilot.ToolInvocation) (copilot.ToolResult, error) {
				args, _ := inv.Arguments.(map[string]any)
				if onEvent != nil {
					onEvent(toolEventFromArgs(inv.ToolName, args))
				}
				text, err := t.Handler(args)
				if err != nil {
					return copilot.ToolResult{}, err
				}
				return copilot.ToolResult{
					TextResultForLLM: text,
					ResultType:       "success",
				}, nil
			},
		}
	}
	return result
}

func toolEventFromArgs(toolName string, args map[string]any) ToolEvent {
	ev := ToolEvent{Name: toolName}
	name := strings.ToLower(toolName)
	switch {
	case strings.Contains(name, "read"):
		ev.Kind = "read"
	case strings.Contains(name, "write") || strings.Contains(name, "edit") || strings.Contains(name, "create"):
		ev.Kind = "write"
	case strings.Contains(name, "bash") || strings.Contains(name, "shell") || strings.Contains(name, "exec") || strings.Contains(name, "run"):
		ev.Kind = "shell"
	case strings.Contains(name, "url") || strings.Contains(name, "fetch") || strings.Contains(name, "http"):
		ev.Kind = "url"
	default:
		ev.Kind = "custom-tool"
	}
	for _, key := range []string{"path", "file_path", "filename", "file", "filepath"} {
		if val, _ := args[key].(string); val != "" {
			ev.Path = val
			break
		}
	}
	return ev
}

func (p *SDKProvider) CompleteWithEvents(ctx context.Context, messages []Message, tools []Tool, onEvent func(ToolEvent)) (string, error) {
	p.once.Do(p.start)
	if p.startErr != nil {
		return "", fmt.Errorf("copilot start: %w", p.startErr)
	}

	var sys, user strings.Builder
	for _, m := range messages {
		switch m.Role {
		case "system":
			sys.WriteString(m.Content)
		case "user":
			user.WriteString(m.Content)
		}
	}

	cfg := &copilot.SessionConfig{
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
		// AvailableTools:      []string{"read", "shell", "url", "bash"},
		Tools: convertTools(tools, onEvent),
	}
	if p.model != "" {
		cfg.Model = p.model
	}
	if sys.Len() > 0 {
		cfg.SystemMessage = &copilot.SystemMessageConfig{
			Mode:    "replace",
			Content: sys.String(),
		}
	}

	session, err := p.client.CreateSession(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	defer session.Disconnect()

	doneCh := make(chan error, 1)
	var content strings.Builder
	var mu sync.Mutex
	unsubscribe := session.On(func(event copilot.SessionEvent) {
		switch d := event.Data.(type) {
		case *copilot.AssistantMessageData:
			mu.Lock()
			content.WriteString(d.Content)
			mu.Unlock()
		case *copilot.SessionIdleData:
			select {
			case doneCh <- nil:
			default:
			}
		case *copilot.SessionErrorData:
			select {
			case doneCh <- fmt.Errorf("session error: %s", d.Message):
			default:
			}
		}
	})
	defer unsubscribe()

	if _, err := session.Send(ctx, copilot.MessageOptions{Prompt: user.String()}); err != nil {
		return "", fmt.Errorf("copilot send: %w", err)
	}

	select {
	case err := <-doneCh:
		if err != nil {
			return "", err
		}
		mu.Lock()
		result := content.String()
		mu.Unlock()
		return result, nil
	case <-ctx.Done():
		return "", fmt.Errorf("copilot timeout: %w", ctx.Err())
	}
}
