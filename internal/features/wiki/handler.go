package wiki

import (
	"context"
	"copilothub/internal/ai"
	"copilothub/internal/knowledge"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	aiTimeout         = 5 * time.Minute
	knowledgeFilesDir = ".spec-designer/knowledge/files"
)

var allowedExts = map[string]string{
	".pdf":  "application/pdf",
	".md":   "text/markdown",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}

type Handler struct {
	rootPath   string
	client     *knowledge.Client
	aiProvider ai.Provider
	topK       int
}

type localProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type chatTurn struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type chatReq struct {
	ProjectPath string     `json:"projectPath"`
	SectionKey  string     `json:"sectionKey"`
	Question    string     `json:"question"`
	History     []chatTurn `json:"history"`
}

type chatResp struct {
	Answer string            `json:"answer"`
	Chunks []knowledge.Chunk `json:"chunks"`
}

func NewHandler(rootPath string, client *knowledge.Client, aiProvider ai.Provider, topK int) *Handler {
	if topK <= 0 {
		topK = 6
	}
	return &Handler{rootPath: rootPath, client: client, aiProvider: aiProvider, topK: topK}
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects := h.localProjects()
	writeJSON(w, map[string]any{"projects": projects})
}

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, "knowledge service is disabled", http.StatusServiceUnavailable)
		return
	}

	var req chatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Question) == "" {
		writeError(w, "question is required", http.StatusBadRequest)
		return
	}
	projectPath := h.resolveProjectPath(req.ProjectPath)
	projectID := projectID(projectPath)
	chunks, err := h.retrieveRelatedChunks(r.Context(), projectID, req.Question)
	if err != nil {
		writeError(w, "knowledge retrieve failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	answer := h.synthesizeAnswer(r.Context(), req.SectionKey, req.Question, req.History, chunks)
	writeJSON(w, chatResp{Answer: answer, Chunks: chunks})
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, "knowledge service is disabled", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	projectPath := h.resolveProjectPath(r.FormValue("projectPath"))
	projectID := projectID(projectPath)
	replaceDuplicates := strings.EqualFold(strings.TrimSpace(r.FormValue("replaceDuplicates")), "true")
	destDir := filepath.Join(projectPath, knowledgeFilesDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		writeError(w, "failed to create storage dir: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fileHeaders := r.MultipartForm.File["files"]
	if len(fileHeaders) == 0 {
		fileHeaders = r.MultipartForm.File["file"]
	}
	if len(fileHeaders) == 0 {
		writeError(w, "file field required", http.StatusBadRequest)
		return
	}

	type uploadResult struct {
		File    string `json:"file"`
		OK      bool   `json:"ok"`
		Message string `json:"message,omitempty"`
	}
	results := make([]uploadResult, 0, len(fileHeaders))

	for _, header := range fileHeaders {
		file, err := header.Open()
		if err != nil {
			results = append(results, uploadResult{File: header.Filename, OK: false, Message: "failed to open file"})
			continue
		}

		ext := strings.ToLower(filepath.Ext(header.Filename))
		contentType, ok := allowedExts[ext]
		if !ok {
			results = append(results, uploadResult{File: header.Filename, OK: false, Message: "unsupported file type"})
			file.Close()
			continue
		}

		destPath := filepath.Join(destDir, header.Filename)

		// Check if file exists and handle duplicate policy
		fileExists := false
		if _, err := os.Stat(destPath); err == nil {
			fileExists = true
		}

		if fileExists && !replaceDuplicates {
			results = append(results, uploadResult{File: header.Filename, OK: false, Message: "duplicate skipped"})
			file.Close()
			continue
		}

		// If replacing, delete old document from vector store first
		if fileExists && replaceDuplicates {
			docs, _ := h.client.ListDocuments(r.Context(), projectID)
			for _, doc := range docs {
				if doc.Name == header.Filename {
					_ = h.client.DeleteDocument(r.Context(), projectID, doc.ID)
					break
				}
			}
		}

		dst, err := os.Create(destPath)
		if err != nil {
			results = append(results, uploadResult{File: header.Filename, OK: false, Message: "failed to save file"})
			file.Close()
			continue
		}
		if _, err := io.Copy(dst, file); err != nil {
			results = append(results, uploadResult{File: header.Filename, OK: false, Message: "failed to write file"})
			dst.Close()
			file.Close()
			continue
		}
		dst.Close()
		file.Close()

		if err := h.client.Ingest(r.Context(), projectID, destPath, header.Filename, contentType); err != nil {
			results = append(results, uploadResult{File: header.Filename, OK: false, Message: "knowledge ingest failed"})
			continue
		}
		results = append(results, uploadResult{File: header.Filename, OK: true})
	}

	writeJSON(w, map[string]any{"results": results})
}

func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeJSON(w, map[string]any{"documents": []knowledge.Document{}})
		return
	}
	projectPath := h.resolveProjectPath(r.URL.Query().Get("projectPath"))
	docs, err := h.client.ListDocuments(r.Context(), projectID(projectPath))
	if err != nil {
		writeError(w, "knowledge service unavailable: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"documents": docs})
}

func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, "knowledge service is disabled", http.StatusServiceUnavailable)
		return
	}
	docID := r.PathValue("id")
	projectPath := h.resolveProjectPath(r.URL.Query().Get("projectPath"))
	if err := h.client.DeleteDocument(r.Context(), projectID(projectPath), docID); err != nil {
		writeError(w, "delete failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (h *Handler) localProjects() []localProject {
	parent := filepath.Dir(h.rootPath)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return []localProject{{ID: projectID(h.rootPath), Name: filepath.Base(h.rootPath), Path: h.rootPath}}
	}
	projects := make([]localProject, 0, len(entries)+1)
	projects = append(projects, localProject{ID: projectID(h.rootPath), Name: filepath.Base(h.rootPath), Path: h.rootPath})
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p := filepath.Join(parent, entry.Name())
		if p == h.rootPath {
			continue
		}
		if _, err := os.Stat(filepath.Join(p, ".git")); err != nil {
			continue
		}
		projects = append(projects, localProject{ID: projectID(p), Name: entry.Name(), Path: p})
	}
	return projects
}

func (h *Handler) resolveProjectPath(projectPath string) string {
	if strings.TrimSpace(projectPath) == "" {
		return h.rootPath
	}
	for _, p := range h.localProjects() {
		if p.Path == projectPath {
			return p.Path
		}
	}
	return h.rootPath
}

func projectID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", sum[:8])
}

func (h *Handler) retrieveRelatedChunks(ctx context.Context, projectID, question string) ([]knowledge.Chunk, error) {
	mainChunks, err := h.client.Retrieve(ctx, projectID, question, h.topK)
	if err != nil {
		return nil, err
	}

	// Extract key terms for broader retrieval
	words := strings.Fields(question)
	var relatedChunks []knowledge.Chunk
	if len(words) > 2 {
		subQuery := strings.Join(words[:len(words)/2], " ")
		if sub, err := h.client.Retrieve(ctx, projectID, subQuery, h.topK/2); err == nil {
			relatedChunks = append(relatedChunks, sub...)
		}
	}

	// Deduplicate by content similarity
	seen := make(map[string]bool)
	var merged []knowledge.Chunk
	for _, chunk := range append(mainChunks, relatedChunks...) {
		key := strings.TrimSpace(chunk.Content)
		if len(key) > 100 {
			key = key[:100]
		}
		if !seen[key] {
			seen[key] = true
			merged = append(merged, chunk)
		}
	}

	if len(merged) > h.topK*2 {
		merged = merged[:h.topK*2]
	}
	return merged, nil
}

func (h *Handler) synthesizeAnswer(ctx context.Context, sectionKey, question string, history []chatTurn, chunks []knowledge.Chunk) string {
	if len(chunks) == 0 {
		return "Không tìm thấy thông tin liên quan trong knowledge của project đã chọn."
	}
	if h.aiProvider == nil {
		return summarizeFromChunks(question, chunks)
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("# Ngữ cảnh hội thoại\n")
	if sectionKey != "" {
		fmt.Fprintf(&contextBuilder, "Section: %s\n", sectionKey)
	}
	if len(history) > 0 {
		contextBuilder.WriteString("\nLịch sử hội thoại gần đây:\n")
		start := 0
		if len(history) > 3 {
			start = len(history) - 3
		}
		for i := start; i < len(history); i++ {
			fmt.Fprintf(&contextBuilder, "Q: %s\nA: %s\n\n", history[i].Question, history[i].Answer)
		}
	}

	contextBuilder.WriteString("\n# Các đoạn knowledge liên quan\n\n")
	for i, chunk := range chunks {
		fmt.Fprintf(&contextBuilder, "--- Đoạn %d (score: %.2f) ---\n%s\n\n", i+1, chunk.Score, strings.TrimSpace(chunk.Content))
	}

	prompt := fmt.Sprintf(`%s
# Câu hỏi hiện tại
%s

# Yêu cầu trả lời
Bạn là Business Analyst và viết theo phong cách NotebookLM: tự nhiên, mạch lạc, dễ đọc, tổng hợp từ nhiều nguồn nhưng không sa đà kỹ thuật.

BẮT BUỘC format output dưới dạng MARKDOWN có cấu trúc rõ ràng:
- Dùng heading cấp 2 (## Tiêu đề) cho từng phần lớn.
- Dùng numbered list cho các bước tuần tự, bullet list cho các ý rời.
- Dùng checkbox “- [ ]” cho checklist xác nhận.
- In đậm (**từ khóa**) cho các khái niệm nghiệp vụ quan trọng.
- Không viết plain text liền mạch không có heading hay list.

Trước khi trả lời, hãy tự xác định câu hỏi thuộc 1 trong 2 case:

## Case A — Hỏi về logic cũ (As-is)
Dùng khi người dùng hỏi cách hệ thống hiện tại đang chạy.
Cấu trúc bắt buộc:
1. ## Tóm tắt nghiệp vụ
2. ## Luồng nghiệp vụ hiện tại (As-is)
3. ## Điều kiện, rẽ nhánh và ngoại lệ
4. ## Điểm BA cần lưu ý

## Case B — Đưa logic mới vào logic cũ (To-be + impact)
Dùng khi người dùng hỏi thêm/chèn/sửa logic mới vào luồng hiện tại.
Cấu trúc bắt buộc:
1. ## Mục tiêu thay đổi nghiệp vụ
2. ## So sánh As-is vs To-be (theo từng bước)
3. ## Impact và điểm xung đột cần xử lý
4. ## Rủi ro nghiệp vụ và đề xuất tích hợp
5. ## Checklist BA xác nhận với PO/Stakeholder (dùng “- [ ]” cho mỗi mục)

Quy tắc chung:
- Không nhắc code/API/database/table/function/column.
- Ưu tiên ngôn ngữ business, ví dụ nghiệp vụ thực tế.
- Nếu dữ liệu chưa đủ, phải có mục “## Phần còn thiếu dữ liệu” và liệt kê câu hỏi cần bổ sung.
`, contextBuilder.String(), question)

	aiCtx, cancel := context.WithTimeout(ctx, aiTimeout)
	defer cancel()

	messages := []ai.Message{{Role: "user", Content: prompt}}
	answer, err := h.aiProvider.Complete(aiCtx, messages)
	if err != nil {
		return fmt.Sprintf("Lỗi khi tổng hợp câu trả lời từ AI: %v", err)
	}
	return strings.TrimSpace(answer)
}

func summarizeFromChunks(question string, chunks []knowledge.Chunk) string {
	if len(chunks) == 0 {
		return "Không tìm thấy thông tin liên quan trong knowledge của project đã chọn."
	}
	var b strings.Builder
	b.WriteString("Dựa trên knowledge của project, thông tin liên quan:\n")
	limit := len(chunks)
	if limit > 3 {
		limit = 3
	}
	for i := 0; i < limit; i++ {
		c := strings.TrimSpace(chunks[i].Content)
		if c == "" {
			continue
		}
		if len(c) > 700 {
			c = c[:700] + "..."
		}
		fmt.Fprintf(&b, "- %s\n", c)
	}
	_ = question
	return strings.TrimSpace(b.String())
}

func aiContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), aiTimeout)
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
