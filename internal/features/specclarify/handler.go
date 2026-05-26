package specclarify

import (
	"context"
	"copilothub/internal/ai"
	"copilothub/internal/ai/tools"
	"copilothub/internal/config"
	"copilothub/internal/knowledge"
	"copilothub/internal/project"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const aiTimeout = 5 * time.Minute

func aiContext(_ *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), aiTimeout)
}

// Handler handles all spec-clarify operations.
type Handler struct {
	cfgStore     *config.Store
	dataDir      string
	projectStore *project.Store

	kcMu  sync.Mutex
	kc    atomic.Pointer[knowledge.Client]
	kcKey string
}

func NewHandler(cfgStore *config.Store, dataDir string, projectStore *project.Store) *Handler {
	return &Handler{
		cfgStore:     cfgStore,
		dataDir:      dataDir,
		projectStore: projectStore,
	}
}

// getKC returns a lazily-initialized knowledge client.
func (h *Handler) getKC() *knowledge.Client {
	cfg, _ := h.cfgStore.Load()
	key := cfg.Knowledge.EmbeddingProvider + "|" + cfg.Knowledge.EmbeddingModel + "|" +
		cfg.Knowledge.EmbeddingKey + "|" + cfg.Knowledge.EmbeddingURL

	if c := h.kc.Load(); c != nil && h.kcKey == key {
		return c
	}

	h.kcMu.Lock()
	defer h.kcMu.Unlock()
	if c := h.kc.Load(); c != nil && h.kcKey == key {
		return c
	}

	storeDir := filepath.Join(h.dataDir, "knowledge-store")
	embedCfg := knowledge.EmbeddingConfig{
		Provider: cfg.Knowledge.EmbeddingProvider,
		Model:    cfg.Knowledge.EmbeddingModel,
		Key:      cfg.Knowledge.EmbeddingKey,
		URL:      cfg.Knowledge.EmbeddingURL,
	}
	client, err := knowledge.NewClient(storeDir, embedCfg)
	if err != nil {
		fmt.Printf("[spec-clarify] knowledge store init failed: %v\n", err)
		return nil
	}
	h.kc.Store(client)
	h.kcKey = key
	return client
}

// provider creates a fresh AI provider from the current config.
func (h *Handler) provider() ai.Provider {
	cfg, _ := h.cfgStore.Load()
	return ai.NewProvider(cfg.AI.Provider, cfg.AI.Token, cfg.AI.Model, cfg.AI.BaseURL, h.dataDir)
}

// providerWithModel creates an AI provider, overriding the model if non-empty.
func (h *Handler) providerWithModel(model string) ai.Provider {
	cfg, _ := h.cfgStore.Load()
	if model == "" {
		model = cfg.AI.Model
	}
	return ai.NewProvider(cfg.AI.Provider, cfg.AI.Token, model, cfg.AI.BaseURL, h.dataDir)
}

// providerWithCwd creates an AI provider with a specific working directory (cwd).
func (h *Handler) providerWithCwd(model, cwd string) ai.Provider {
	cfg, _ := h.cfgStore.Load()
	if model == "" {
		model = cfg.AI.Model
	}
	return ai.NewProvider(cfg.AI.Provider, cfg.AI.Token, model, cfg.AI.BaseURL, cwd)
}

type clarifyReq struct {
	Spec        string   `json:"spec"`
	Mode        string   `json:"mode"` // "source" | "wiki"
	WikiContent string   `json:"wikiContent"`
	ProjectPath string   `json:"projectPath"`
	ProjectID   string   `json:"projectId"`
	RepoIDs     []string `json:"repoIds"` // empty = all repos
	Model       string   `json:"model,omitempty"`
}

type fileRef struct {
	Path string `json:"path"`
	URL  string `json:"url,omitempty"`
}

type clarifyIssue struct {
	ID              string    `json:"id"`
	Category        string    `json:"category"`
	Severity        string    `json:"severity"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Suggestion      string    `json:"suggestion"`
	ReferencedFiles []fileRef `json:"referenced_files,omitempty"`
	WikiSections    []string  `json:"wiki_sections,omitempty"`
}

type clarifyResponse struct {
	Summary   string         `json:"summary"`
	Issues    []clarifyIssue `json:"issues"`
	SessionID string         `json:"sessionId,omitempty"`
}

func (h *Handler) Clarify(w http.ResponseWriter, r *http.Request) {
	var req clarifyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Spec) == "" {
		writeError(w, "spec is required", http.StatusBadRequest)
		return
	}

	var systemPrompt string
	var prompt strings.Builder

	// Retrieve relevant code from vector index when in source/both mode.
	var codeContext string
	if (req.Mode == "source" || req.Mode == "both" || req.Mode == "") && req.ProjectID != "" {
		if kc := h.getKC(); kc != nil {
			ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
			chunks, err := kc.RetrieveCode(ctx2, req.ProjectID, req.RepoIDs, req.Spec, 30)
			cancel2()
			if err == nil && len(chunks) > 0 {
				var sb strings.Builder
				sb.WriteString("## Relevant source code (retrieved from codebase index)\n\n")
				for _, c := range chunks {
					header := fmt.Sprintf("### File: %s", c.FilePath)
					if c.Language != "" {
						header += fmt.Sprintf(" [%s]", c.Language)
					}
					if len(c.SymbolNames) > 0 {
						limit := 5
						if len(c.SymbolNames) < limit {
							limit = len(c.SymbolNames)
						}
						header += fmt.Sprintf(" — contains: %s", strings.Join(c.SymbolNames[:limit], ", "))
					}
					header += fmt.Sprintf(" (relevance: %.2f)", c.Score)
					sb.WriteString(header + "\n```\n" + c.Content + "\n```\n\n")
				}
				codeContext = sb.String()
			}
		}
	}

	switch req.Mode {
	case "wiki":
		if strings.TrimSpace(req.WikiContent) == "" {
			writeError(w, "wikiContent is required for wiki mode", http.StatusBadRequest)
			return
		}
		systemPrompt = clarifyWithWikiPrompt
		fmt.Fprintf(&prompt, "Spec document:\n%s\n\n", req.Spec)
		fmt.Fprintf(&prompt, "Wiki/Documentation:\n%s\n\n", req.WikiContent)
		prompt.WriteString("Analyze the spec against the wiki content. Identify issues and generate Q&A for ambiguous points.")
	case "both":
		if strings.TrimSpace(req.WikiContent) == "" {
			writeError(w, "wikiContent is required for both mode", http.StatusBadRequest)
			return
		}
		systemPrompt = clarifyWithBothPrompt
		if codeContext != "" {
			fmt.Fprintf(&prompt, "%s\n\n", codeContext)
		}
		fmt.Fprintf(&prompt, "Spec document:\n%s\n\n", req.Spec)
		fmt.Fprintf(&prompt, "Wiki/Documentation:\n%s\n\n", req.WikiContent)
		prompt.WriteString("Cross-reference spec với source code ở trên VÀ wiki. Xác định tất cả vấn đề.")
	default: // "source"
		systemPrompt = clarifyWithSourcePrompt
		if codeContext != "" {
			fmt.Fprintf(&prompt, "%s\n\n", codeContext)
		}
		fmt.Fprintf(&prompt, "Spec document:\n%s\n\n", req.Spec)
		prompt.WriteString("Analyze the spec against the source code provided above. Identify issues based on actual implementation.")
	}

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt.String()},
	}

	ctx, cancel := aiContext(r)
	defer cancel()

	// Build repo dir→metadata map for file URL resolution.
	var repoDirs map[string]project.Repository
	if (req.Mode == "source" || req.Mode == "both") && req.ProjectID != "" && h.projectStore != nil {
		repoDirs = h.projectStore.ReposWithSourceDirs(req.ProjectID, req.RepoIDs)
	}

	// Single AI call — code context already retrieved via embeddings above
	p := h.providerWithModel(req.Model)

	if csp, ok := p.(ai.ChatSessionProvider); ok {
		result, sessionID, err := csp.CompleteWithSession(ctx, messages, nil, nil)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp := parseClarifyResult(result, repoDirs)
		resp.SessionID = sessionID
		writeJSON(w, resp)
		return
	}

	if ep, ok := p.(ai.EventingProvider); ok {
		result, err := ep.CompleteWithEvents(ctx, messages, nil, nil)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, parseClarifyResult(result, repoDirs))
		return
	}

	result, err := p.Complete(ctx, messages)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, parseClarifyResult(result, repoDirs))
}

func sseWriter(w http.ResponseWriter) (send func(string, any), canFlush bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	send = func(event string, data any) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		if ok {
			flusher.Flush()
		}
	}
	return send, ok
}

// injectRepoContext prepends a repository context header to the first user message
// so the AI knows the exact root path to use in referenced_files.
func injectRepoContext(messages []ai.Message, repoName, repoDir string) []ai.Message {
	header := fmt.Sprintf("[Repository context]\nName: %s\nRoot directory: %s\nAll paths in referenced_files must be relative to this root (e.g. \"internal/api/handler.go:10-25\", never absolute).\n\n", repoName, repoDir)
	out := make([]ai.Message, len(messages))
	copy(out, messages)
	for i, m := range out {
		if m.Role == "user" {
			out[i].Content = header + m.Content
			break
		}
	}
	return out
}

// clarifyMultiRepo runs clarification for each repo sequentially and merges results.
func (h *Handler) clarifyMultiRepo(w http.ResponseWriter, ctx context.Context, messages []ai.Message, repoDirs map[string]project.Repository, model string) {
	var allIssues []clarifyIssue
	var summaries []string
	var lastSessionID string

	for repoDir, repo := range repoDirs {
		p := h.providerWithCwd(model, repoDir)
		repoMessages := injectRepoContext(messages, repo.Name, repoDir)

		csp, ok := p.(ai.ChatSessionProvider)
		if !ok {
			// Fallback to simple completion
			result, err := p.Complete(ctx, repoMessages)
			if err != nil {
				continue
			}
			resp := parseClarifyResult(result, repoDirs)
			for i := range resp.Issues {
				resp.Issues[i].Title = fmt.Sprintf("[%s] %s", repo.Name, resp.Issues[i].Title)
			}
			allIssues = append(allIssues, resp.Issues...)
			if resp.Summary != "" {
				summaries = append(summaries, fmt.Sprintf("[%s] %s", repo.Name, resp.Summary))
			}
			continue
		}

		result, sessionID, err := csp.CompleteWithSession(ctx, repoMessages, nil, nil)
		if err != nil {
			continue
		}
		lastSessionID = sessionID

		resp := parseClarifyResult(result, repoDirs)
		for i := range resp.Issues {
			resp.Issues[i].Title = fmt.Sprintf("[%s] %s", repo.Name, resp.Issues[i].Title)
		}
		allIssues = append(allIssues, resp.Issues...)
		if resp.Summary != "" {
			summaries = append(summaries, fmt.Sprintf("[%s] %s", repo.Name, resp.Summary))
		}
	}

	// Re-number issue IDs
	for i := range allIssues {
		allIssues[i].ID = fmt.Sprintf("i%d", i+1)
	}

	combinedResp := clarifyResponse{
		Summary:   strings.Join(summaries, "\n"),
		Issues:    allIssues,
		SessionID: lastSessionID,
	}
	if combinedResp.Issues == nil {
		combinedResp.Issues = []clarifyIssue{}
	}
	writeJSON(w, combinedResp)
}

// aiClarifyIssue mirrors clarifyIssue but keeps referenced_files as []string
// since that is what the AI emits; resolved to []fileRef after parsing.
type aiClarifyIssue struct {
	ID              string   `json:"id"`
	Category        string   `json:"category"`
	Severity        string   `json:"severity"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Suggestion      string   `json:"suggestion"`
	ReferencedFiles []string `json:"referenced_files,omitempty"`
	WikiSections    []string `json:"wiki_sections,omitempty"`
}

type aiClarifyResponse struct {
	Summary string           `json:"summary"`
	Issues  []aiClarifyIssue `json:"issues"`
}

func parseClarifyResult(raw string, repoDirs map[string]project.Repository) clarifyResponse {
	var ai aiClarifyResponse
	if err := json.Unmarshal([]byte(cleanJSON(raw)), &ai); err != nil {
		return clarifyResponse{Issues: []clarifyIssue{}}
	}
	resp := clarifyResponse{Summary: ai.Summary}
	for _, iss := range ai.Issues {
		ci := clarifyIssue{
			ID:           iss.ID,
			Category:     iss.Category,
			Severity:     iss.Severity,
			Title:        iss.Title,
			Description:  iss.Description,
			Suggestion:   iss.Suggestion,
			WikiSections: iss.WikiSections,
		}
		for _, raw := range iss.ReferencedFiles {
			ci.ReferencedFiles = append(ci.ReferencedFiles, resolveFileRef(raw, repoDirs))
		}
		resp.Issues = append(resp.Issues, ci)
	}
	if resp.Issues == nil {
		resp.Issues = []clarifyIssue{}
	}
	return resp
}

// resolveFileRef parses "path/to/file.go:10-25" and resolves a GitHub URL
// by stat-checking each repo clone dir.
func resolveFileRef(raw string, repoDirs map[string]project.Repository) fileRef {
	ref := fileRef{Path: raw}
	// Parse optional line range suffix: "file.go:10-25" or "file.go:10"
	filePath := raw
	lines := ""
	if idx := strings.LastIndex(raw, ":"); idx > 0 {
		suffix := raw[idx+1:]
		if isLineRange(suffix) {
			filePath = raw[:idx]
			lines = suffix
		}
	}
	for dir, repo := range repoDirs {
		if _, err := os.Stat(filepath.Join(dir, filePath)); err == nil {
			ref.URL = buildGitHubURL(repo.RepoURL, repo.RepoBranch, filePath, lines)
			return ref
		}
	}
	return ref
}

func isLineRange(s string) bool {
	for _, c := range s {
		if c != '-' && (c < '0' || c > '9') {
			return false
		}
	}
	return len(s) > 0
}

func buildGitHubURL(repoURL, branch, path, lines string) string {
	base := strings.TrimSuffix(repoURL, ".git")
	base = strings.TrimSuffix(base, "/")
	base = strings.Replace(base, "git@github.com:", "https://github.com/", 1)
	if branch == "" {
		branch = "main"
	}
	anchor := ""
	if lines != "" {
		if idx := strings.Index(lines, "-"); idx >= 0 {
			anchor = "#L" + lines[:idx] + "-L" + lines[idx+1:]
		} else {
			anchor = "#L" + lines
		}
	}
	return fmt.Sprintf("%s/blob/%s/%s%s", base, branch, path, anchor)
}

// === Helpers ===

func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

// === Chat (persistent session) ===

type chatReq struct {
	SessionID string   `json:"sessionId"`
	Message   string   `json:"message"`
	ProjectID string   `json:"projectId,omitempty"`
	RepoIDs   []string `json:"repoIds,omitempty"`
	Model     string   `json:"model,omitempty"`
}

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	var req chatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.SessionID == "" || strings.TrimSpace(req.Message) == "" {
		writeError(w, "sessionId and message are required", http.StatusBadRequest)
		return
	}

	p := h.providerWithModel(req.Model)
	csp, ok := p.(ai.ChatSessionProvider)
	if !ok {
		writeError(w, "current AI provider does not support persistent sessions", http.StatusNotImplemented)
		return
	}

	var sourceTools []ai.Tool
	if req.ProjectID != "" && h.projectStore != nil {
		repoDirs := h.projectStore.ReposWithSourceDirs(req.ProjectID, req.RepoIDs)
		var paths []string
		for dir := range repoDirs {
			if dir != "" {
				paths = append(paths, dir)
			}
		}
		if len(paths) > 0 {
			sourceTools = tools.SourceScanTools(paths)
		}
	}

	ctx, cancel := aiContext(r)
	defer cancel()

	sendEvent, _ := sseWriter(w)

	result, err := csp.ChatWithSession(ctx, req.SessionID, req.Message, sourceTools, func(ev ai.ToolEvent) {
		sendEvent("tool", ev)
	})
	if err != nil {
		sendEvent("error", map[string]string{"error": err.Error()})
		return
	}
	sendEvent("message", map[string]string{"content": result})
}
