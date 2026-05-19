package specclarify

import (
	"context"
	"copilothub/internal/ai"
	"copilothub/internal/repo"
	"encoding/json"
	"fmt"
	"io"
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
	provider ai.Provider
	scanner  *repo.Scanner
}

func NewHandler(provider ai.Provider, repoPath string) *Handler {
	return &Handler{
		provider: provider,
		scanner:  repo.NewScanner(repoPath),
	}
}

// === Clarify Spec ===

const clarifyWithSourcePrompt = `You are an expert Business Analyst and Software Architect with direct access to the project workspace.

Task: Analyze the provided spec/requirement against the actual source code to identify gaps, conflicts, and ambiguities.

Use your file reading tools to:
1. Read relevant source files based on the project structure below
2. Identify existing data models, API endpoints, and business rules
3. Compare the spec against what is actually implemented

Output MUST be valid JSON with this structure:
{
  "summary": "Đánh giá tổng quan ngắn gọn trong 1-2 câu",
  "issues": [
    {
      "id": "i1",
      "category": "gap|conflict|ambiguity|suggestion",
      "severity": "high|medium|low",
      "title": "Tiêu đề ngắn của vấn đề",
      "description": "Mô tả chi tiết vấn đề",
      "suggestion": "Cách khắc phục hoặc làm rõ"
    }
  ],
  "questions": [
    {
      "id": "q1",
      "question": "Câu hỏi cần user xác nhận",
      "context": "Lý do cần hỏi / thông tin liên quan từ spec hoặc code",
      "options": ["Option A", "Option B"],
      "defaultAnswer": "Câu trả lời gợi ý"
    }
  ]
}

Category meanings:
- gap: Spec yêu cầu điều gì đó chưa có trong codebase
- conflict: Spec mâu thuẫn với code hiện tại
- ambiguity: Spec không rõ khi so với implementation hiện có
- suggestion: Đề xuất cải tiến dựa trên pattern của code

Rules for questions:
- Chỉ tạo question khi spec thực sự mập mờ và cần user confirm
- Mỗi question nên có context rõ ràng (trích từ spec hoặc code)
- Cung cấp options nếu có thể, kèm defaultAnswer gợi ý
- Tối đa 5 questions, ưu tiên các vấn đề quan trọng nhất

Output ONLY valid JSON. Language is Vietnamese.`

const clarifyWithWikiPrompt = `You are an expert Business Analyst reviewing a spec against wiki/documentation.

Task: Compare the spec/requirement with the provided wiki content to find gaps, conflicts, and inconsistencies.

Output MUST be valid JSON with this structure:
{
  "summary": "Đánh giá tổng quan ngắn gọn trong 1-2 câu",
  "issues": [
    {
      "id": "i1",
      "category": "gap|conflict|ambiguity|suggestion",
      "severity": "high|medium|low",
      "title": "Tiêu đề ngắn",
      "description": "Mô tả chi tiết vấn đề",
      "suggestion": "Cách khắc phục"
    }
  ],
  "questions": [
    {
      "id": "q1",
      "question": "Câu hỏi cần user xác nhận",
      "context": "Lý do cần hỏi / trích từ spec hoặc wiki",
      "options": ["Option A", "Option B"],
      "defaultAnswer": "Câu trả lời gợi ý"
    }
  ]
}

Category meanings:
- gap: Spec đề cập điều gì đó wiki không tài liệu hóa, hoặc ngược lại
- conflict: Spec mâu thuẫn với tài liệu wiki
- ambiguity: Thuật ngữ hoặc khái niệm không nhất quán
- suggestion: Đề xuất cải thiện để align với wiki

Rules for questions:
- Chỉ tạo question khi thực sự cần user confirm do spec hoặc wiki mập mờ
- Cung cấp context cụ thể (trích từ spec/wiki)
- Cung cấp options nếu có thể, kèm defaultAnswer gợi ý
- Tối đa 5 questions

Output ONLY valid JSON. Language is Vietnamese.`

type clarifyReq struct {
	Spec        string `json:"spec"`
	Mode        string `json:"mode"` // "source" | "wiki"
	WikiContent string `json:"wikiContent"`
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
		info, _ := h.scanner.Scan()
		appendRepoContext(&prompt, info)
		fmt.Fprintf(&prompt, "Spec document:\n%s\n\n", req.Spec)
		prompt.WriteString("Analyze the spec against the source code. Identify issues and generate Q&A for ambiguous points.")
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

// === Fetch Wiki ===

type fetchWikiReq struct {
	URL string `json:"url"`
}

type fetchWikiResponse struct {
	Content string `json:"content"`
	Title   string `json:"title"`
}

func (h *Handler) FetchWiki(w http.ResponseWriter, r *http.Request) {
	var req fetchWikiReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeError(w, "url is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		writeError(w, "invalid URL: "+err.Error(), http.StatusBadRequest)
		return
	}
	httpReq.Header.Set("User-Agent", "spec-clarify/1.0")
	httpReq.Header.Set("Accept", "text/html,text/plain,application/xhtml+xml")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		writeError(w, "failed to fetch URL: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 500_000))
	if err != nil {
		writeError(w, "failed to read response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	content := stripHTMLTags(string(body))
	writeJSON(w, fetchWikiResponse{Content: content, Title: req.URL})
}

// === Helpers ===

func appendRepoContext(b *strings.Builder, info *repo.Info) {
	if info == nil {
		return
	}
	fmt.Fprintf(b, "Repository: %s\nTech stack: %s\n\n", info.Name, strings.Join(info.TechStack, ", "))
	if len(info.FileTree) > 0 {
		b.WriteString("### Project Structure\n")
		b.WriteString(repo.FormatTree(info.FileTree, 0))
		b.WriteString("\n")
	}
}

func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, ch := range s {
		switch {
		case ch == '<':
			inTag = true
		case ch == '>':
			inTag = false
			b.WriteRune(' ')
		case !inTag:
			b.WriteRune(ch)
		}
	}
	result := b.String()
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(result)
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
