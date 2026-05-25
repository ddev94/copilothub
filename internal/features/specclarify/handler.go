package specclarify

import (
	"context"
	"copilothub/internal/ai"
	"copilothub/internal/ai/tools"
	"copilothub/internal/config"
	"copilothub/internal/project"
	"copilothub/internal/repo"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
}

func NewHandler(cfgStore *config.Store, dataDir string, projectStore *project.Store) *Handler {
	return &Handler{
		cfgStore:     cfgStore,
		dataDir:      dataDir,
		projectStore: projectStore,
	}
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

// === Clarify Spec ===

const clarifyWithSourcePrompt = `You are a senior Business Analyst / BRSE doing a final review of a spec before it is handed to developers.

Your mission: Find everything that would cause a developer to be confused, blocked, or to implement the wrong thing.
Think like the developer who will read this spec tomorrow and has to implement it from scratch.

You have file reading tools. Before analyzing, explore the codebase: read API routes, data models, validation logic, and relevant handlers to understand actual behavior. Code is ground truth — never suggest code changes.

## What to check (in order of importance)

1. MISSING FLOWS: Are all required user flows described? Happy path + alternative paths + error paths?
2. MISSING EDGE CASES: What happens when input is empty/invalid? When a resource doesn't exist? When permissions are denied? When an external service fails?
3. MISSING CONSTRAINTS: What are the validation rules? Character limits? Allowed values? Business rules?
4. AMBIGUITY: Any requirement a developer could interpret in 2+ different ways? Any vague verbs ("handle", "manage", "process") without concrete definition?
5. INACCURACY: Does the spec describe something that contradicts what the code actually does?

## Output format

Output MUST be valid JSON:
{
  "summary": "1-2 câu: spec này có sẵn sàng giao dev chưa? Điểm yếu chính là gì?",
  "issues": [
    {
      "id": "i1",
      "category": "missing_flow|missing_edge_case|missing_constraint|ambiguity|inaccuracy",
      "severity": "high|medium|low",
      "title": "Tên ngắn của vấn đề",
      "description": "Mô tả cụ thể: spec đang viết gì (hoặc không viết gì), tại sao dev sẽ bị block/stuck ở đây",
      "suggestion": "Viết sẵn text cần thêm hoặc sửa vào spec. Ví dụ: 'Thêm vào spec: [text cụ thể]' hoặc 'Sửa \"[text cũ]\" thành \"[text mới]\"'",
      "referenced_files": ["path/to/file.go:10-25"]
    }
  ]
}

## Category meanings
- missing_flow: Một luồng người dùng hoàn toàn vắng mặt trong spec (dev không biết phải xử lý case này)
- missing_edge_case: Trường hợp đặc biệt chưa được mô tả (input rỗng, lỗi, permission, timeout, v.v.)
- missing_constraint: Rule validation, giới hạn, hoặc điều kiện nghiệp vụ chưa được specify
- ambiguity: Yêu cầu quá mơ hồ — một dev có thể hiểu thành 2+ cách khác nhau
- inaccuracy: Spec mô tả sai so với behavior thực tế của code

## Rules for "suggestion" field
- PHẢI cụ thể — viết sẵn đoạn text để BA copy-paste vào spec
- Format: "Thêm vào spec: '[text]'" hoặc "Sửa '[text cũ]' thành '[text mới]'"
- Nếu tìm thấy behavior thực tế từ code, thêm dòng "📎 Ref: <path/to/file.go> (line X)" vào cuối suggestion
- KHÔNG viết chung chung như "làm rõ thêm" hay "bổ sung thông tin"
- KHÔNG bao giờ gợi ý sửa code

## Rules for "referenced_files" field
- List the source files you actually read that are directly relevant to THIS specific issue
- Use relative paths from the repository root
- Omit if no source file is relevant to this issue

Output ONLY valid JSON. Language is Vietnamese.`

const clarifyWithBothPrompt = `You are a senior Business Analyst / BRSE doing a final review of a spec before it is handed to developers.

Your mission: Find everything that would cause a developer to be confused, blocked, or to implement the wrong thing.
Think like the developer who will read this spec tomorrow and has to implement it from scratch.

You have BOTH source code access AND wiki/documentation. Use them together:
- You have file reading tools — explore the codebase to verify actual behavior. Code is ground truth for what the system currently does.
- Wiki/documentation is provided as reference for business rules and domain knowledge. Wiki is ground truth for business intent — never suggest wiki changes.

## What to check (in order of importance)

1. MISSING FLOWS: Are all required user flows described? Happy path + alternative paths + error paths?
2. MISSING EDGE CASES: What happens when input is empty/invalid? When a resource doesn't exist? When permissions are denied? When an external service fails?
3. MISSING CONSTRAINTS: What are the validation rules? Character limits? Allowed values? Business rules from wiki not reflected in spec?
4. AMBIGUITY: Any requirement a developer could interpret in 2+ different ways?
5. INACCURACY: Does the spec contradict what the code actually does OR misrepresent what the wiki documents?
6. CODE-WIKI CONFLICT: Does the source code do something DIFFERENT from what the wiki documents? Surface this — do NOT pick a side.

## Output format

Output MUST be valid JSON:
{
  "summary": "1-2 câu: spec này có sẵn sàng giao dev chưa? Điểm yếu chính là gì?",
  "issues": [
    {
      "id": "i1",
      "category": "missing_flow|missing_edge_case|missing_constraint|ambiguity|inaccuracy|code_wiki_conflict",
      "severity": "high|medium|low",
      "title": "Tên ngắn của vấn đề",
      "description": "Mô tả cụ thể: spec đang viết gì (hoặc không viết gì), tại sao dev sẽ bị block/stuck ở đây",
      "suggestion": "Viết sẵn text cần thêm hoặc sửa vào spec. Ví dụ: 'Thêm vào spec: [text cụ thể]' hoặc 'Sửa \"[text cũ]\" thành \"[text mới]\"'",
      "referenced_files": ["path/to/file.go:10-25"],
      "wiki_sections": ["Tên section/heading trong wiki liên quan đến issue này"]
    }
  ]
}

## Category meanings
- missing_flow: Một luồng người dùng hoàn toàn vắng mặt trong spec
- missing_edge_case: Trường hợp đặc biệt chưa được mô tả
- missing_constraint: Rule validation, giới hạn, hoặc điều kiện nghiệp vụ chưa được specify
- ambiguity: Yêu cầu quá mơ hồ — một dev có thể hiểu thành 2+ cách khác nhau
- inaccuracy: Spec mô tả sai so với code thực tế HOẶC sai so với wiki (nhưng code và wiki đồng nhất)
- code_wiki_conflict: Code đang làm X nhưng wiki quy định Y — hai nguồn mâu thuẫn nhau. KHÔNG tự phán xét ai đúng. Spec cần BA xác nhận follow cái nào.

## Rules for "suggestion" field
- PHẢI cụ thể — viết sẵn đoạn text để BA copy-paste vào spec
- Format: "Thêm vào spec: '[text]'" hoặc "Sửa '[text cũ]' thành '[text mới]'"
- Nếu tìm thấy behavior từ code, thêm "📎 Ref: <path/to/file.go> (line X)" vào cuối suggestion
- Nếu lấy từ wiki, thêm "📖 Wiki: [tên section]" vào cuối suggestion
- KHÔNG viết chung chung như "làm rõ thêm" hay "bổ sung thông tin"
- KHÔNG bao giờ gợi ý sửa code hoặc wiki
- Với code_wiki_conflict: suggestion phải nêu rõ "Code đang làm [X], wiki quy định [Y] — BA cần xác nhận spec follow cái nào trước khi dev thực hiện"

## Rules for referenced_files / wiki_sections per issue
- referenced_files: list các source files bạn đọc liên quan đến issue này; include line numbers khi có thể, format: "path/to/file.go:10-25" (omit if none)
- wiki_sections: list các heading/section wiki bạn dùng làm căn cứ cho issue này (omit if none)

Output ONLY valid JSON. Language is Vietnamese.`

const clarifyWithWikiPrompt = `You are a senior Business Analyst / BRSE doing a final review of a spec before it is handed to developers.

Your mission: Find everything that would cause a developer to be confused, blocked, or to implement the wrong thing.
Think like the developer who will read this spec tomorrow and has to implement it from scratch.

The wiki/documentation is provided as reference to verify accuracy. Wiki is ground truth — never suggest wiki changes.

## What to check (in order of importance)

1. MISSING FLOWS: Are all required user flows described? Happy path + alternative paths + error paths?
2. MISSING EDGE CASES: What happens when input is empty/invalid? When a resource doesn't exist? When permissions are denied?
3. MISSING CONSTRAINTS: What are the validation rules? Character limits? Allowed values? Business rules defined in wiki?
4. AMBIGUITY: Any requirement a developer could interpret in 2+ different ways?
5. INACCURACY: Does the spec contradict or misrepresent what the wiki documents?

## Output format

Output MUST be valid JSON:
{
  "summary": "1-2 câu: spec này có sẵn sàng giao dev chưa? Điểm yếu chính là gì?",
  "issues": [
    {
      "id": "i1",
      "category": "missing_flow|missing_edge_case|missing_constraint|ambiguity|inaccuracy",
      "severity": "high|medium|low",
      "title": "Tên ngắn của vấn đề",
      "description": "Mô tả cụ thể: spec đang viết gì (hoặc không viết gì), tại sao dev sẽ bị block/stuck ở đây",
      "suggestion": "Viết sẵn text cần thêm hoặc sửa vào spec. Ví dụ: 'Thêm vào spec: [text cụ thể]' hoặc 'Sửa \"[text cũ]\" thành \"[text mới]\"'",
      "wiki_sections": ["Tên section/heading trong wiki liên quan đến issue này"]
    }
  ]
}

## Category meanings
- missing_flow: Một luồng người dùng hoàn toàn vắng mặt trong spec
- missing_edge_case: Trường hợp đặc biệt chưa được mô tả (input rỗng, lỗi, permission, v.v.)
- missing_constraint: Rule validation, giới hạn, hoặc điều kiện nghiệp vụ từ wiki chưa được spec đề cập
- ambiguity: Yêu cầu quá mơ hồ — một dev có thể hiểu thành 2+ cách khác nhau
- inaccuracy: Spec mâu thuẫn hoặc mô tả sai so với wiki

## Rules for "suggestion" field
- PHẢI cụ thể — viết sẵn đoạn text để BA copy-paste vào spec
- Format: "Thêm vào spec: '[text]'" hoặc "Sửa '[text cũ]' thành '[text mới]'"
- KHÔNG viết chung chung như "làm rõ thêm" hay "bổ sung thông tin"
- KHÔNG bao giờ gợi ý sửa wiki

## Rules for "wiki_sections" per issue
- List heading/section names from the wiki that you used as the basis for this specific issue (omit if none)

Output ONLY valid JSON. Language is Vietnamese.`

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
	var sourcePaths []string

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
		if req.ProjectPath != "" {
			sourcePaths = []string{req.ProjectPath}
		} else if req.ProjectID != "" && h.projectStore != nil {
			sourcePaths = h.projectStore.SourceDirsForRepos(req.ProjectID, req.RepoIDs)
		}
		for _, path := range sourcePaths {
			scanner := repo.NewScanner(path)
			info, _ := scanner.Scan()
			appendRepoContext(&prompt, path, info)
		}
		prompt.WriteString("STEP 1 — Bắt buộc: dùng file reading tools để khám phá repository ở trên. Đọc các route, model, handler, validation liên quan đến spec trước khi phân tích.\n\n")
		fmt.Fprintf(&prompt, "STEP 2 — Spec document:\n%s\n\n", req.Spec)
		fmt.Fprintf(&prompt, "STEP 3 — Wiki/Documentation (business rules reference):\n%s\n\n", req.WikiContent)
		prompt.WriteString("Bây giờ cross-reference spec với cả source code bạn vừa đọc VÀ wiki ở trên. Xác định tất cả vấn đề.")
	default: // "source"
		systemPrompt = clarifyWithSourcePrompt
		if req.ProjectPath != "" {
			sourcePaths = []string{req.ProjectPath}
		} else if req.ProjectID != "" && h.projectStore != nil {
			sourcePaths = h.projectStore.SourceDirsForRepos(req.ProjectID, req.RepoIDs)
		}
		for _, path := range sourcePaths {
			scanner := repo.NewScanner(path)
			info, _ := scanner.Scan()
			appendRepoContext(&prompt, path, info)
		}
		fmt.Fprintf(&prompt, "Spec document:\n%s\n\n", req.Spec)
		prompt.WriteString("Use your file reading tools to explore the repository paths above, then analyze the spec. Read actual source files to verify behavior before identifying issues.")
	}

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt.String()},
	}

	ctx, cancel := aiContext(r)
	defer cancel()

	// Build source scanning tools and repo dir→metadata map for file URL resolution.
	var sourceTools []ai.Tool
	var repoDirs map[string]project.Repository
	if req.Mode == "source" || req.Mode == "both" {
		if len(sourcePaths) > 0 {
			sourceTools = tools.SourceScanTools(sourcePaths)
		}
		if req.ProjectID != "" && h.projectStore != nil {
			repoDirs = h.projectStore.ReposWithSourceDirs(req.ProjectID, req.RepoIDs)
		}
	}

	p := h.providerWithModel(req.Model)
	isSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream")

	if csp, ok := p.(ai.ChatSessionProvider); ok {
		if isSSE {
			h.clarifySSE(w, ctx, messages, sourceTools, repoDirs, csp)
		} else {
			result, sessionID, err := csp.CompleteWithSession(ctx, messages, sourceTools, nil)
			if err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			resp := parseClarifyResult(result, repoDirs)
			resp.SessionID = sessionID
			writeJSON(w, resp)
		}
		return
	}

	if ep, ok := p.(ai.EventingProvider); ok {
		if isSSE {
			h.clarifySSELegacy(w, ctx, messages, sourceTools, repoDirs, ep)
		} else {
			result, err := ep.CompleteWithEvents(ctx, messages, sourceTools, nil)
			if err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, parseClarifyResult(result, repoDirs))
		}
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

func (h *Handler) clarifySSE(w http.ResponseWriter, ctx context.Context, messages []ai.Message, sourceTools []ai.Tool, repoDirs map[string]project.Repository, csp ai.ChatSessionProvider) {
	sendEvent, _ := sseWriter(w)

	result, sessionID, err := csp.CompleteWithSession(ctx, messages, sourceTools, func(ev ai.ToolEvent) {
		sendEvent("tool", ev)
	})
	if err != nil {
		sendEvent("error", map[string]string{"error": err.Error()})
		return
	}
	resp := parseClarifyResult(result, repoDirs)
	resp.SessionID = sessionID
	sendEvent("result", resp)
}

func (h *Handler) clarifySSELegacy(w http.ResponseWriter, ctx context.Context, messages []ai.Message, sourceTools []ai.Tool, repoDirs map[string]project.Repository, ep ai.EventingProvider) {
	sendEvent, _ := sseWriter(w)

	result, err := ep.CompleteWithEvents(ctx, messages, sourceTools, func(ev ai.ToolEvent) {
		sendEvent("tool", ev)
	})
	if err != nil {
		sendEvent("error", map[string]string{"error": err.Error()})
		return
	}
	sendEvent("result", parseClarifyResult(result, repoDirs))
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

// === Refine Spec ===

const refineSystemPrompt = `You are a senior Business Analyst rewriting a spec to make it ready for development.

CONTEXT: A spec has been reviewed and issues were found. Your job is to rewrite the spec so a developer can implement it without ambiguity or blockers.
The source code / wiki is always correct — only the spec needs to change.

You will receive:
1. The original spec document
2. Specific issues found (missing flows, missing edge cases, missing constraints, ambiguity, inaccuracy)
3. BA/BRSE answers to clarification questions

Your job:
- For each issue: rewrite, add, or remove the specific part of the spec to fix it
- For missing_flow: add the missing flow with enough detail for a developer to implement
- For missing_edge_case: add explicit handling for the edge case (what should happen, what error to show, etc.)
- For missing_constraint: add the specific rule/validation/limit to the relevant requirement
- For ambiguity: rewrite the vague requirement with precise, unambiguous language
- For inaccuracy: correct the spec to match actual behavior
- Incorporate BA/BRSE answers to fill in business logic decisions
- Write in the same language as the original spec

## Output format
- Output the rewritten spec in **Markdown format**
- Use "##" / "###" for sections, "-" for bullet lists, **bold** for key terms/constraints
- Do NOT wrap the output in a code block
- Output ONLY the rewritten spec. No explanations, no preamble.`

type refineReq struct {
	Spec    string            `json:"spec"`
	Issues  []clarifyIssue    `json:"issues"`
	Answers map[string]string `json:"answers"`
	Model   string            `json:"model,omitempty"`
}

type refineResponse struct {
	RefinedSpec string `json:"refinedSpec"`
}

func (h *Handler) Refine(w http.ResponseWriter, r *http.Request) {
	var req refineReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Spec) == "" {
		writeError(w, "spec is required", http.StatusBadRequest)
		return
	}

	var prompt strings.Builder
	fmt.Fprintf(&prompt, "Original spec:\n%s\n\n", req.Spec)

	if len(req.Issues) > 0 {
		prompt.WriteString("Issues found during analysis:\n")
		for _, issue := range req.Issues {
			fmt.Fprintf(&prompt, "- [%s/%s] %s: %s\n  Suggestion: %s\n",
				issue.Category, issue.Severity, issue.Title, issue.Description, issue.Suggestion)
		}
		prompt.WriteString("\n")
	}

	if len(req.Answers) > 0 {
		prompt.WriteString("User's answers to clarification questions:\n")
		for id, answer := range req.Answers {
			fmt.Fprintf(&prompt, "- Question %s: %s\n", id, answer)
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("Please rewrite the spec to fix all issues and incorporate the answers above.")

	ctx, cancel := aiContext(r)
	defer cancel()

	result, err := h.providerWithModel(req.Model).Complete(ctx, []ai.Message{
		{Role: "system", Content: refineSystemPrompt},
		{Role: "user", Content: prompt.String()},
	})
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, refineResponse{RefinedSpec: strings.TrimSpace(result)})
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

func appendRepoContext(b *strings.Builder, path string, info *repo.Info) {
	if info == nil {
		fmt.Fprintf(b, "Repository path: %s\n\n", path)
		return
	}
	fmt.Fprintf(b, "Repository: %s\nPath: %s\nTech stack: %s\n\n", info.Name, path, strings.Join(info.TechStack, ", "))
	if len(info.FileTree) > 0 {
		b.WriteString("### Project Structure\n")
		b.WriteString(repo.FormatTree(info.FileTree, 0))
		b.WriteString("\n")
	}
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
