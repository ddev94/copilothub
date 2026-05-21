package specclarify

import (
	"context"
	"copilothub/internal/ai"
	"copilothub/internal/project"
	"copilothub/internal/repo"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const aiTimeout = 5 * time.Minute

func aiContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), aiTimeout)
}

// Handler handles all spec-clarify operations.
type Handler struct {
	provider     ai.Provider
	projectStore *project.Store
}

func NewHandler(provider ai.Provider, projectStore *project.Store) *Handler {
	return &Handler{
		provider:     provider,
		projectStore: projectStore,
	}
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
      "suggestion": "Viết sẵn text cần thêm hoặc sửa vào spec. Ví dụ: 'Thêm vào spec: [text cụ thể]' hoặc 'Sửa \"[text cũ]\" thành \"[text mới]\"'"
    }
  ],
  "questions": [
    {
      "id": "q1",
      "issueId": "i1",
      "question": "Câu hỏi cần BA/BRSE xác nhận để điền vào chỗ còn thiếu trong spec",
      "context": "Trích dẫn phần spec liên quan hoặc behavior thực tế từ code",
      "options": ["Option A", "Option B"],
      "defaultAnswer": "Gợi ý dựa trên pattern của codebase"
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

## Rules for questions
- Chỉ tạo question khi BA/BRSE là người duy nhất có thể trả lời (quyết định nghiệp vụ)
- Mỗi question PHẢI có issueId
- Tối đa 5 questions, ưu tiên high severity
- Không hỏi những gì đã rõ từ code

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
      "suggestion": "Viết sẵn text cần thêm hoặc sửa vào spec. Ví dụ: 'Thêm vào spec: [text cụ thể]' hoặc 'Sửa \"[text cũ]\" thành \"[text mới]\"'"
    }
  ],
  "questions": [
    {
      "id": "q1",
      "issueId": "i1",
      "question": "Câu hỏi cần BA/BRSE xác nhận để điền vào chỗ còn thiếu trong spec",
      "context": "Trích dẫn phần spec liên quan hoặc nội dung wiki liên quan",
      "options": ["Option A", "Option B"],
      "defaultAnswer": "Gợi ý dựa trên wiki"
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

## Rules for questions
- Chỉ tạo question khi BA/BRSE là người duy nhất có thể trả lời (quyết định nghiệp vụ)
- Mỗi question PHẢI có issueId
- Tối đa 5 questions, ưu tiên high severity

Output ONLY valid JSON. Language is Vietnamese.`

type clarifyReq struct {
	Spec        string   `json:"spec"`
	Mode        string   `json:"mode"` // "source" | "wiki"
	WikiContent string   `json:"wikiContent"`
	ProjectPath string   `json:"projectPath"`
	ProjectID   string   `json:"projectId"`
	RepoIDs     []string `json:"repoIds"` // empty = all repos
}

type clarifyIssue struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

type clarifyQuestion struct {
	ID            string   `json:"id"`
	IssueID       string   `json:"issueId,omitempty"`
	Question      string   `json:"question"`
	Context       string   `json:"context"`
	Options       []string `json:"options"`
	DefaultAnswer string   `json:"defaultAnswer"`
}

type clarifyResponse struct {
	Issues    []clarifyIssue    `json:"issues"`
	Questions []clarifyQuestion `json:"questions"`
	Summary   string            `json:"summary"`
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
	default: // "source"
		systemPrompt = clarifyWithSourcePrompt
		var sourcePaths []string
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

	ctx, cancel := aiContext(r)
	defer cancel()

	result, err := h.provider.Complete(ctx, []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt.String()},
	})
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cleaned := cleanJSON(result)
	var resp clarifyResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		writeError(w, fmt.Sprintf("failed to parse AI response: %v", err), http.StatusInternalServerError)
		return
	}
	if resp.Issues == nil {
		resp.Issues = []clarifyIssue{}
	}
	if resp.Questions == nil {
		resp.Questions = []clarifyQuestion{}
	}
	writeJSON(w, resp)
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
- Preserve the original spec's structure and format
- Write in the same language as the original spec

Output ONLY the rewritten spec text. Do NOT include explanations, JSON, or markdown code blocks.`

type refineReq struct {
	Spec    string            `json:"spec"`
	Issues  []clarifyIssue    `json:"issues"`
	Answers map[string]string `json:"answers"`
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

	result, err := h.provider.Complete(ctx, []ai.Message{
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
