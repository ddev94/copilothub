package specdesigner

import (
	"context"
	"copilothub/internal/ai"
	"copilothub/internal/features/specdesigner/spec"
	"copilothub/internal/repo"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

const aiTimeout = 5 * time.Minute

func aiContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), aiTimeout)
}

// SpecHandler handles spec CRUD operations.
type SpecHandler struct {
	store *spec.Store
}

func NewSpecHandler(repoPath string) *SpecHandler {
	return &SpecHandler{store: spec.NewStore(repoPath)}
}

func (h *SpecHandler) List(w http.ResponseWriter, r *http.Request) {
	metas, err := h.store.List()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, metas)
}

func (h *SpecHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.store.Load(id)
	if os.IsNotExist(err) {
		writeError(w, "spec not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s)
}

func (h *SpecHandler) Save(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var s spec.Spec
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.ID = id
	if err := h.store.Save(&s); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s)
}

func (h *SpecHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Accept an optional full Spec body (e.g. AI-generated); otherwise create blank.
	var s spec.Spec
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil || s.ID == "" {
		blank := h.store.NewDefault()
		if s.Title != "" {
			blank.Title = s.Title
		}
		s = *blank
	}
	if err := h.store.Save(&s); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, s)
}

func (h *SpecHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.Delete(id); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// AIHandler handles AI-powered operations.
type AIHandler struct {
	provider ai.Provider
	scanner  *repo.Scanner
}

func NewAIHandler(provider ai.Provider, repoPath string) *AIHandler {
	return &AIHandler{
		provider: provider,
		scanner:  repo.NewScanner(repoPath),
	}
}

const userStorySystemPrompt = `You are an expert Business Analyst with direct access to the project workspace.

Before generating, use your file reading tools to explore the codebase: read key source files based on the project structure provided, identify data models, API definitions, and existing conventions. Use this understanding to produce user stories that are consistent with the actual implementation.

Output MUST be valid JSON with this exact structure:
{
  "userStories": [
    {
      "title": "Short title",
      "story": "As a [role], I want [feature], so that [benefit]",
      "acceptanceCriteria": [
        {"given": "context or precondition", "when": "user action or event", "then": "expected outcome"}
      ],
      "testCases": [
        {"title": "Test case title", "steps": "Step-by-step instructions", "expectedResult": "Expected outcome"}
      ]
    }
  ],
  "relevantFiles": [
    {"path": "relative/path/to/file.go", "reason": "Why this file is relevant to the requirement"}
  ]
}

Rules:
- Each user story must follow the "As a... I want... So that..." format.
- Acceptance criteria must use structured given/when/then fields (not a single description string).
- Test cases must have clear steps and expected results.
- Generate comprehensive but focused stories — not too many, not too few.
- relevantFiles: list every source file you read that informed the output. Include relative paths.
- Output ONLY the JSON, no markdown fences, no explanation.
- Language is Vietnamese. Use Vietnamese for all generated content.`

// suggestReq improves or generates content for a single section.
type suggestReq struct {
	Requirement string `json:"requirement"`
	Context     string `json:"context"`
}

type relevantFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type suggestResponse struct {
	Content       string          `json:"content"`
	RelevantFiles []relevantFile  `json:"relevantFiles"`
}

func (h *AIHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	var req suggestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, _ := h.scanner.Scan()

	messages := []ai.Message{
		{Role: "system", Content: userStorySystemPrompt},
		{Role: "user", Content: buildSuggestPrompt(req.Requirement, req.Context, info)},
	}

	ctx, cancel := aiContext(r)
	defer cancel()
	result, err := h.provider.Complete(ctx, messages)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cleaned := cleanJSON(result)
	var parsed struct {
		RelevantFiles []relevantFile `json:"relevantFiles"`
	}
	_ = json.Unmarshal([]byte(cleaned), &parsed)

	writeJSON(w, suggestResponse{
		Content:       cleaned,
		RelevantFiles: parsed.RelevantFiles,
	})
}

// Clarify analyzes a requirement for ambiguity and returns questions.
const clarifySystemPrompt = `You are an expert Business Analyst with direct access to the project workspace.

Before analyzing, use your file reading tools to read relevant source files, data models, and API definitions. Use this context to ask smarter, more targeted questions — avoid asking about things that are already answered by the existing code.

If the requirement is clear enough to generate user stories, respond with:
{"clear": true, "questions": []}

If the requirement is vague or missing important details, respond with:
{"clear": false, "questions": [{"id": "q1", "question": "Your question here", "suggestion": "A possible default answer or hint"}]}

Rules:
- Ask only questions that are truly necessary to generate good user stories.
- Reference actual file or type names you found in the codebase when relevant (e.g. "Is this the same User model in internal/models/user.go?").
- Maximum 5 questions. Focus on the most impactful ambiguities.
- Each question should have a helpful suggestion as a default/hint.
- Questions should cover: target users, scope boundaries, key constraints, expected behaviors.
- Output ONLY valid JSON, no markdown fences, no explanation.
- Language is Vietnamese. Use Vietnamese for all generated content.`

type clarifyReq struct {
	Requirement string `json:"requirement"`
}

type clarifyQuestion struct {
	ID         string `json:"id"`
	Question   string `json:"question"`
	Suggestion string `json:"suggestion"`
}

type clarifyResponse struct {
	Clear     bool              `json:"clear"`
	Questions []clarifyQuestion `json:"questions"`
}

func (h *AIHandler) Clarify(w http.ResponseWriter, r *http.Request) {
	var req clarifyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Requirement) == "" {
		writeError(w, "requirement is required", http.StatusBadRequest)
		return
	}

	info, _ := h.scanner.Scan()

	var prompt strings.Builder
	appendRepoContext(&prompt, info)
	fmt.Fprintf(&prompt, "Requirement:\n%s\n\nAnalyze this requirement for ambiguity and missing details.", req.Requirement)

	ctx, cancel := aiContext(r)
	defer cancel()
	result, err := h.provider.Complete(ctx, []ai.Message{
		{Role: "system", Content: clarifySystemPrompt},
		{Role: "user", Content: prompt.String()},
	})
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cleaned := cleanJSON(result)

	var resp clarifyResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		writeJSON(w, clarifyResponse{Clear: true, Questions: []clarifyQuestion{}})
		return
	}

	writeJSON(w, resp)
}

// generateSpecReq creates user stories from a requirement description.
type generateSpecReq struct {
	Title         string `json:"title"`
	Requirement   string `json:"requirement"`
	Clarification string `json:"clarification"`
}

func (h *AIHandler) GenerateSpec(w http.ResponseWriter, r *http.Request) {
	var req generateSpecReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := aiContext(r)
	defer cancel()
	repoInfo, _ := h.scanner.Scan()

	prompt := buildGeneratePrompt(req, repoInfo)

	result, err := h.provider.Complete(ctx, []ai.Message{
		{Role: "system", Content: userStorySystemPrompt},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse the AI response into user stories
	var parsed struct {
		UserStories []struct {
			Title              string `json:"title"`
			Story              string `json:"story"`
			AcceptanceCriteria []struct {
				Given string `json:"given"`
				When  string `json:"when"`
				Then  string `json:"then"`
			} `json:"acceptanceCriteria"`
			TestCases []struct {
				Title          string `json:"title"`
				Steps          string `json:"steps"`
				ExpectedResult string `json:"expectedResult"`
			} `json:"testCases"`
		} `json:"userStories"`
	}

	// Clean up potential markdown fences
	cleaned := strings.TrimSpace(result)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		writeError(w, fmt.Sprintf("failed to parse AI response: %v", err), http.StatusInternalServerError)
		return
	}

	userStories := make([]spec.UserStory, 0, len(parsed.UserStories))
	for _, s := range parsed.UserStories {
		criteria := make([]spec.AcceptanceCriterion, 0, len(s.AcceptanceCriteria))
		for _, ac := range s.AcceptanceCriteria {
			criteria = append(criteria, spec.AcceptanceCriterion{
				ID:    uuid.NewString(),
				Given: ac.Given,
				When:  ac.When,
				Then:  ac.Then,
			})
		}
		testCases := make([]spec.TestCase, 0, len(s.TestCases))
		for _, tc := range s.TestCases {
			testCases = append(testCases, spec.TestCase{
				ID:             uuid.NewString(),
				Title:          tc.Title,
				Steps:          tc.Steps,
				ExpectedResult: tc.ExpectedResult,
			})
		}
		userStories = append(userStories, spec.UserStory{
			ID:                 uuid.NewString(),
			Title:              s.Title,
			Story:              s.Story,
			AcceptanceCriteria: criteria,
			TestCases:          testCases,
		})
	}

	now := time.Now()
	title := req.Title
	if title == "" {
		title = "User Stories"
	}

	writeJSON(w, spec.Spec{
		ID:          uuid.NewString(),
		Title:       title,
		Version:     "1.0.0",
		CreatedAt:   now,
		UpdatedAt:   now,
		Requirement: req.Requirement,
		UserStories: userStories,
	})
}

// appendRepoContext writes repo metadata and file tree into b as orientation for Copilot.
// Copilot already has filesystem access via cwd; the tree helps it navigate efficiently.
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

func buildSuggestPrompt(requirement, context string, info *repo.Info) string {
	var b strings.Builder
	appendRepoContext(&b, info)
	if context != "" {
		fmt.Fprintf(&b, "Existing context:\n%s\n\n", context)
	}
	fmt.Fprintf(&b, "Requirement:\n%s\n\nGenerate user stories with acceptance criteria and test cases.", requirement)
	return b.String()
}

func buildGeneratePrompt(req generateSpecReq, info *repo.Info) string {
	var b strings.Builder
	appendRepoContext(&b, info)
	fmt.Fprintf(&b, "Requirement:\n%s\n", req.Requirement)
	if req.Clarification != "" {
		fmt.Fprintf(&b, "\nAdditional clarification:\n%s\n", req.Clarification)
	}
	b.WriteString("\nGenerate comprehensive user stories with acceptance criteria and test cases for this requirement.")
	return b.String()
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// JSON helpers
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
