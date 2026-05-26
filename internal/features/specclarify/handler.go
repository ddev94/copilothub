package specclarify

import (
	"context"
	"copilothub/internal/ai"
	aitools "copilothub/internal/ai/tools"
	"copilothub/internal/config"
	"copilothub/internal/knowledge"
	"copilothub/internal/project"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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

	// Build repo dir→metadata map (needed for both URL resolution and source tools).
	var repoDirs map[string]project.Repository
	if (req.Mode == "source" || req.Mode == "both") && req.ProjectID != "" && h.projectStore != nil {
		repoDirs = h.projectStore.ReposWithSourceDirs(req.ProjectID, req.RepoIDs)
	}

	// Hybrid retrieval: vector DB narrows the search to candidate files; AI then
	// uses read_file to inspect them and pinpoint exact line ranges.
	var fileListContext string
	if (req.Mode == "source" || req.Mode == "both" || req.Mode == "") && req.ProjectID != "" {
		if kc := h.getKC(); kc != nil {
			maxFiles := maxCandidateFiles(len(repoDirs), false)
			ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
			chunks, err := kc.RetrieveCode(ctx2, req.ProjectID, req.RepoIDs, req.Spec, maxFiles*3)
			cancel2()
			if err == nil && len(chunks) > 0 {
				candidates := candidateFilesFromChunks(chunks, maxFiles)
				fileListContext = buildFileListContext(candidates)
			}
		}
	}

	// Build source-scan tools so the AI can read the actual files.
	var sourceTools []ai.Tool
	if len(repoDirs) > 0 {
		var paths []string
		for dir := range repoDirs {
			if dir != "" {
				paths = append(paths, dir)
			}
		}
		if len(paths) > 0 {
			sourceTools = aitools.SourceScanTools(paths)
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
		if fileListContext != "" {
			fmt.Fprintf(&prompt, "%s\n\n", fileListContext)
		}
		fmt.Fprintf(&prompt, "Spec document:\n%s\n\n", req.Spec)
		fmt.Fprintf(&prompt, "Wiki/Documentation:\n%s\n\n", req.WikiContent)
		prompt.WriteString("Use the read_file tool to inspect candidate files, cross-reference with spec and wiki, then list issues.")
	default: // "source"
		systemPrompt = clarifyWithSourcePrompt
		if fileListContext != "" {
			fmt.Fprintf(&prompt, "%s\n\n", fileListContext)
		}
		fmt.Fprintf(&prompt, "Spec document:\n%s\n\n", req.Spec)
		prompt.WriteString("Use the read_file tool to inspect candidate files, identify issues, and cite exact line numbers shown by read_file.")
	}

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt.String()},
	}

	ctx, cancel := aiContext(r)
	defer cancel()

	p := h.providerWithModel(req.Model)

	if csp, ok := p.(ai.ChatSessionProvider); ok {
		toolEventCount := 0
		onEvent := func(ev ai.ToolEvent) {
			toolEventCount++
			fmt.Printf("[clarify-debug] tool[%d] %s\n", toolEventCount, ev.Name)
		}
		result, sessionID, err := csp.CompleteWithSession(ctx, messages, sourceTools, onEvent)
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
		result, err := ep.CompleteWithEvents(ctx, messages, sourceTools, nil)
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

// === Candidate file helpers (hybrid retrieval) ===

// candidateFile is a file proposed for AI inspection, aggregated from chunks.
type candidateFile struct {
	Path     string
	Language string
	Score    float64
	Symbols  []string
}

// maxCandidateFiles returns how many top candidate files to surface, scaled by
// the number of repos being scanned.
//
// stream=true allows a higher ceiling because files are analysed one-by-one and
// the full content never lands in a single prompt.
// stream=false keeps the list tighter because all file paths go into one AI prompt.
func maxCandidateFiles(numRepos int, stream bool) int {
	if numRepos < 1 {
		numRepos = 1
	}
	if stream {
		n := numRepos * 15
		if n < 15 {
			n = 15
		}
		if n > 40 {
			n = 40
		}
		return n
	}
	n := numRepos * 8
	if n < 10 {
		n = 10
	}
	if n > 20 {
		n = 20
	}
	return n
}

// candidateFilesFromChunks groups retrieved chunks by file path, accumulates the
// best score per file, and returns the list sorted by relevance descending.
func candidateFilesFromChunks(chunks []knowledge.CodeChunk, maxFiles int) []candidateFile {
	idx := make(map[string]*candidateFile)
	var order []string
	for _, c := range chunks {
		key := c.FilePath
		if key == "" {
			continue
		}
		cf, ok := idx[key]
		if !ok {
			cf = &candidateFile{Path: key, Language: c.Language}
			idx[key] = cf
			order = append(order, key)
		}
		if c.Score > cf.Score {
			cf.Score = c.Score
		}
		cf.Symbols = mergeUniqueStrings(cf.Symbols, c.SymbolNames)
	}
	out := make([]candidateFile, 0, len(order))
	for _, k := range order {
		out = append(out, *idx[k])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > maxFiles {
		out = out[:maxFiles]
	}
	return out
}

// buildFileListContext renders the candidate file list for the AI prompt.
func buildFileListContext(files []candidateFile) string {
	if len(files) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Candidate files (ranked by relevance to the spec)\n\n")
	sb.WriteString("Use the read_file tool to inspect each file. The tool returns content with line-number prefixes (\"  NN | code\") that you MUST use when citing referenced_files.\n\n")
	for i, f := range files {
		fmt.Fprintf(&sb, "%d. `%s`", i+1, f.Path)
		if f.Language != "" {
			fmt.Fprintf(&sb, " [%s]", f.Language)
		}
		if len(f.Symbols) > 0 {
			shown := f.Symbols
			if len(shown) > 8 {
				shown = shown[:8]
			}
			fmt.Fprintf(&sb, " — contains: %s", strings.Join(shown, ", "))
		}
		fmt.Fprintf(&sb, " (relevance: %.2f)\n", f.Score)
	}
	return sb.String()
}

// === Code context helpers ===

// chunkWithLines pairs a code chunk's content with its starting line number.
type chunkWithLines struct {
	content   string
	startLine int // 0 = unknown
	endLine   int
	score     float64
}

// maxChunksPerFile limits how many chunks of the same file are shown to the AI.
// Keeping it small prevents the union of chunks from covering the whole file,
// which would let the AI cite a wide ":1-end" range instead of a specific section.
const maxChunksPerFile = 2

// fileCodeEntry accumulates chunks from the same file.
type fileCodeEntry struct {
	filePath    string
	language    string
	symbolNames []string
	chunks      []chunkWithLines
	maxScore    float64
}

// buildCodeContext groups retrieved code chunks by file and renders each file as a
// single annotated code block with per-line numbers so the AI can cite exact lines.
func buildCodeContext(chunks []knowledge.CodeChunk) string {
	fileMap := make(map[string]*fileCodeEntry)
	var fileOrder []string

	for _, c := range chunks {
		key := c.FilePath
		if key == "" {
			key = "(unknown)"
		}
		entry, exists := fileMap[key]
		if !exists {
			entry = &fileCodeEntry{
				filePath: c.FilePath,
				language: c.Language,
			}
			fileMap[key] = entry
			fileOrder = append(fileOrder, key)
		}
		entry.chunks = append(entry.chunks, chunkWithLines{
			content:   c.Content,
			startLine: c.StartLine,
			endLine:   c.EndLine,
			score:     c.Score,
		})
		entry.symbolNames = mergeUniqueStrings(entry.symbolNames, c.SymbolNames)
		if c.Score > entry.maxScore {
			entry.maxScore = c.Score
		}
	}

	// Cap chunks per file to the top-N by score, so we never cover the whole file.
	for _, e := range fileMap {
		if len(e.chunks) > maxChunksPerFile {
			sort.SliceStable(e.chunks, func(i, j int) bool {
				return e.chunks[i].score > e.chunks[j].score
			})
			e.chunks = e.chunks[:maxChunksPerFile]
		}
	}

	var sb strings.Builder
	sb.WriteString("## Relevant source code (retrieved from codebase index)\n\n")
	for _, key := range fileOrder {
		e := fileMap[key]
		// Sort chunks by start line so the code reads top-to-bottom.
		sort.SliceStable(e.chunks, func(i, j int) bool {
			return e.chunks[i].startLine < e.chunks[j].startLine
		})
		header := fmt.Sprintf("### File: %s", e.filePath)
		if e.language != "" {
			header += fmt.Sprintf(" [%s]", e.language)
		}
		if len(e.symbolNames) > 0 {
			shown := e.symbolNames
			if len(shown) > 5 {
				shown = shown[:5]
			}
			header += fmt.Sprintf(" — contains: %s", strings.Join(shown, ", "))
		}
		header += fmt.Sprintf(" (relevance: %.2f)", e.maxScore)
		rendered := renderChunksWithLineNumbers(e.chunks)
		sb.WriteString(header + "\n```" + e.language + "\n" + rendered + "\n```\n\n")
	}
	return sb.String()
}

// renderChunksWithLineNumbers renders a file's chunks as a single code block with
// line-number prefixes (e.g. "  23 | const x = 1"). Non-contiguous chunks are
// separated by an explicit gap marker so the AI knows the sections are independent.
func renderChunksWithLineNumbers(chunks []chunkWithLines) string {
	var sb strings.Builder
	seen := make(map[string]bool)
	var prevEnd int
	first := true

	for _, ch := range chunks {
		// Strip the "// File: ...\n" header that chunkCodeFile prepends to every chunk.
		body := ch.content
		if nl := strings.Index(body, "\n"); nl >= 0 && strings.HasPrefix(body, "// File:") {
			body = body[nl+1:]
		}
		body = strings.TrimRight(body, "\n")
		if strings.TrimSpace(body) == "" {
			continue
		}

		// Dedup by first 80 bytes.
		fp := body
		if len(fp) > 80 {
			fp = fp[:80]
		}
		if seen[fp] {
			continue
		}
		seen[fp] = true

		if !first {
			gapFrom := prevEnd + 1
			gapTo := ch.startLine - 1
			if ch.startLine > 0 && prevEnd > 0 && gapTo >= gapFrom {
				fmt.Fprintf(&sb, "\n// ─── lines %d–%d not shown (different section) ───\n\n", gapFrom, gapTo)
			} else {
				sb.WriteString("\n// ─── different section below ───\n\n")
			}
		}
		first = false

		lines := strings.Split(body, "\n")
		lineNum := ch.startLine
		for _, line := range lines {
			if lineNum > 0 {
				fmt.Fprintf(&sb, "%4d | %s\n", lineNum, line)
				lineNum++
			} else {
				sb.WriteString(line + "\n")
			}
		}
		if ch.endLine > 0 {
			prevEnd = ch.endLine
		} else if ch.startLine > 0 {
			prevEnd = ch.startLine + strings.Count(body, "\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// === Per-file streaming clarify ===

// ClarifyStream scans candidate files one-by-one and streams issues via SSE.
// When the AI provider supports ChatSessionProvider (e.g. GitHub Copilot SDK),
// all file turns share a single session so the AI retains full context across files.
//
// SSE events:
//
//	start    {"totalFiles": N}
//	scanning {"file": "...", "language": "...", "index": N, "total": N}
//	issues   {"file": "...", "issues": [...]}
//	done     {"totalIssues": N, "sessionId": "..."}
func (h *Handler) ClarifyStream(w http.ResponseWriter, r *http.Request) {
	var req clarifyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Spec) == "" {
		writeError(w, "spec is required", http.StatusBadRequest)
		return
	}

	var repoDirs map[string]project.Repository
	if req.ProjectID != "" && h.projectStore != nil {
		repoDirs = h.projectStore.ReposWithSourceDirs(req.ProjectID, req.RepoIDs)
	}

	var candidates []candidateFile
	var chunksByFile map[string][]knowledge.CodeChunk
	if req.ProjectID != "" {
		if kc := h.getKC(); kc != nil {
			maxFiles := maxCandidateFiles(len(repoDirs), true)
			ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
			chunks, err := kc.RetrieveCode(ctx2, req.ProjectID, req.RepoIDs, req.Spec, maxFiles*3)
			cancel2()
			if err == nil && len(chunks) > 0 {
				candidates, chunksByFile = candidateFilesAndChunks(chunks, maxFiles)
			}
		}
	}

	send, _ := sseWriter(w)

	if len(candidates) == 0 {
		send("done", map[string]any{"totalIssues": 0})
		return
	}

	ctx, cancel := aiContext(r)
	defer cancel()

	p := h.providerWithModel(req.Model)
	csp, hasSession := p.(ai.ChatSessionProvider)

	send("start", map[string]any{"totalFiles": len(candidates)})

	var sessionID string
	totalIssues := 0

	for i, cf := range candidates {
		if ctx.Err() != nil {
			break
		}

		send("scanning", map[string]any{
			"file":     cf.Path,
			"language": cf.Language,
			"index":    i + 1,
			"total":    len(candidates),
		})

		repoDir, repo := findRepoForFile(cf.Path, repoDirs)
		content := ""
		if repoDir != "" {
			content = readFileWithLineNumbers(filepath.Join(repoDir, cf.Path))
		}
		if content == "" {
			if fileChunks, ok := chunksByFile[cf.Path]; ok {
				content = renderCodeChunks(fileChunks)
			}
		}
		if content == "" {
			send("issues", map[string]any{"file": cf.Path, "issues": []clarifyIssue{}})
			continue
		}

		var raw string
		var callErr error

		switch {
		case hasSession && sessionID == "":
			// First file: start a new session — send system prompt + spec + file.
			messages := []ai.Message{
				{Role: "system", Content: clarifyPerFilePrompt},
				{Role: "user", Content: buildFirstFileMessage(req.Spec, cf, content, repo.Name, len(candidates))},
			}
			raw, sessionID, callErr = csp.CompleteWithSession(ctx, messages, nil, nil)

		case hasSession && sessionID != "":
			// Subsequent files: resume session, spec already in conversation history.
			raw, callErr = csp.ChatWithSession(ctx, sessionID, buildNextFileMessage(cf, content, i+1, len(candidates)), nil, nil)

		default:
			// Provider does not support sessions: fall back to stateless completion.
			messages := []ai.Message{
				{Role: "system", Content: clarifyPerFilePrompt},
				{Role: "user", Content: buildFirstFileMessage(req.Spec, cf, content, repo.Name, 1)},
			}
			raw, callErr = p.Complete(ctx, messages)
		}

		if callErr != nil {
			fmt.Printf("[clarify-stream] file %s error: %v\n", cf.Path, callErr)
			send("issues", map[string]any{"file": cf.Path, "issues": []clarifyIssue{}})
			continue
		}

		issues := parseRawIssues(raw, repoDirs)
		totalIssues += len(issues)
		send("issues", map[string]any{"file": cf.Path, "issues": issues})
	}

	send("done", map[string]any{"totalIssues": totalIssues, "sessionId": sessionID})
}

// buildFirstFileMessage builds the user message for the first file in a session.
// It includes the full spec so the AI can reference it across all subsequent turns.
func buildFirstFileMessage(spec string, cf candidateFile, content, repoName string, totalFiles int) string {
	var sb strings.Builder
	if repoName != "" {
		fmt.Fprintf(&sb, "Repository: %s\n\n", repoName)
	}
	if totalFiles > 1 {
		fmt.Fprintf(&sb, "We will scan %d files. For each, output only {\"issues\":[...]} JSON.\n\n", totalFiles)
	}
	fmt.Fprintf(&sb, "Spec document:\n%s\n\n", spec)
	writeFileBlock(&sb, cf, content)
	return sb.String()
}

// buildNextFileMessage builds the user message for subsequent files in an existing session.
// The spec is already in the conversation history — no need to repeat it.
func buildNextFileMessage(cf candidateFile, content string, index, total int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "File %d/%d:\n", index, total)
	writeFileBlock(&sb, cf, content)
	return sb.String()
}

// writeFileBlock writes a file header + content block into sb.
func writeFileBlock(sb *strings.Builder, cf candidateFile, content string) {
	fmt.Fprintf(sb, "File: %s", cf.Path)
	if cf.Language != "" {
		fmt.Fprintf(sb, " [%s]", cf.Language)
	}
	if len(cf.Symbols) > 0 {
		shown := cf.Symbols
		if len(shown) > 10 {
			shown = shown[:10]
		}
		fmt.Fprintf(sb, "\nDefines: %s", strings.Join(shown, ", "))
	}
	fmt.Fprintf(sb, "\n\n```\n%s\n```", content)
}

// parseRawIssues parses the AI's JSON response into clarifyIssues, resolving file refs.
func parseRawIssues(raw string, repoDirs map[string]project.Repository) []clarifyIssue {
	type perFileResp struct {
		Issues []aiClarifyIssue `json:"issues"`
	}
	var resp perFileResp
	if err := json.Unmarshal([]byte(cleanJSON(raw)), &resp); err != nil {
		return nil
	}
	issues := make([]clarifyIssue, 0, len(resp.Issues))
	for _, iss := range resp.Issues {
		ci := clarifyIssue{
			ID:          iss.ID,
			Category:    iss.Category,
			Severity:    iss.Severity,
			Title:       iss.Title,
			Description: iss.Description,
			Suggestion:  iss.Suggestion,
		}
		for _, raw := range iss.ReferencedFiles {
			ci.ReferencedFiles = append(ci.ReferencedFiles, resolveFileRef(raw, repoDirs))
		}
		issues = append(issues, ci)
	}
	return issues
}

// analyzeFileForSpec calls the AI (stateless) to find spec issues in a single file.
// Used as a fallback when the provider does not support ChatSessionProvider.
func (h *Handler) analyzeFileForSpec(ctx context.Context, spec string, cf candidateFile, content, repoName, model string, repoDirs map[string]project.Repository) []clarifyIssue {
	messages := []ai.Message{
		{Role: "system", Content: clarifyPerFilePrompt},
		{Role: "user", Content: buildFirstFileMessage(spec, cf, content, repoName, 1)},
	}
	p := h.providerWithModel(model)
	result, err := p.Complete(ctx, messages)
	if err != nil {
		return nil
	}
	return parseRawIssues(result, repoDirs)
}

// candidateFilesAndChunks groups chunks by file, returning both the ordered
// candidate list and a map of chunks per file path for fallback content.
func candidateFilesAndChunks(chunks []knowledge.CodeChunk, maxFiles int) ([]candidateFile, map[string][]knowledge.CodeChunk) {
	idx := make(map[string]*candidateFile)
	byFile := make(map[string][]knowledge.CodeChunk)
	var order []string

	for _, c := range chunks {
		key := c.FilePath
		if key == "" {
			continue
		}
		cf, ok := idx[key]
		if !ok {
			cf = &candidateFile{Path: key, Language: c.Language}
			idx[key] = cf
			order = append(order, key)
		}
		if c.Score > cf.Score {
			cf.Score = c.Score
		}
		cf.Symbols = mergeUniqueStrings(cf.Symbols, c.SymbolNames)
		byFile[key] = append(byFile[key], c)
	}

	out := make([]candidateFile, 0, len(order))
	for _, k := range order {
		out = append(out, *idx[k])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > maxFiles {
		out = out[:maxFiles]
	}
	return out, byFile
}

// findRepoForFile returns the repo dir and metadata for the repo that contains filePath.
func findRepoForFile(filePath string, repoDirs map[string]project.Repository) (string, project.Repository) {
	for dir, repo := range repoDirs {
		if _, err := os.Stat(filepath.Join(dir, filePath)); err == nil {
			return dir, repo
		}
	}
	return "", project.Repository{}
}

// readFileWithLineNumbers reads a file and returns its content with "  NN | line" prefixes.
// Returns empty string if the file cannot be read or exceeds 512KB.
// Caps output at 1000 lines to keep AI context manageable.
func readFileWithLineNumbers(absPath string) string {
	info, err := os.Stat(absPath)
	if err != nil || info.Size() > 512*1024 || info.Size() == 0 {
		return ""
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	const maxLines = 1000
	truncated := false
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}
	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, "%4d | %s\n", i+1, line)
	}
	if truncated {
		fmt.Fprintf(&sb, "... (truncated at line %d)\n", maxLines)
	}
	return sb.String()
}

// renderCodeChunks renders vector-DB chunks as line-numbered content for use
// when the file on disk is unavailable (e.g. remote repo not cloned).
func renderCodeChunks(chunks []knowledge.CodeChunk) string {
	if len(chunks) == 0 {
		return ""
	}
	// Sort by start line so code reads top-to-bottom.
	sort.SliceStable(chunks, func(i, j int) bool {
		return chunks[i].StartLine < chunks[j].StartLine
	})
	cwl := make([]chunkWithLines, len(chunks))
	for i, c := range chunks {
		cwl[i] = chunkWithLines{content: c.Content, startLine: c.StartLine, endLine: c.EndLine, score: c.Score}
	}
	return renderChunksWithLineNumbers(cwl)
}

// mergeUniqueStrings appends items from src into dst, skipping duplicates.
func mergeUniqueStrings(dst, src []string) []string {
	seen := make(map[string]bool, len(dst))
	for _, s := range dst {
		seen[s] = true
	}
	for _, s := range src {
		if !seen[s] {
			seen[s] = true
			dst = append(dst, s)
		}
	}
	return dst
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
			sourceTools = aitools.SourceScanTools(paths)
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
