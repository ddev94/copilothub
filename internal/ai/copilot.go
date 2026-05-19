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

	reply, err := session.SendAndWait(ctx, copilot.MessageOptions{
		Prompt: user.String(),
	})
	if err != nil {
		return "", fmt.Errorf("copilot send: %w", err)
	}
	if reply == nil {
		return "", fmt.Errorf("no response from Copilot")
	}
	d, ok := reply.Data.(*copilot.AssistantMessageData)
	if !ok {
		return "", fmt.Errorf("unexpected response type: %T", reply.Data)
	}
	return d.Content, nil
}
