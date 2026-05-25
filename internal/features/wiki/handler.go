package wiki

import (
	"context"
	"copilothub/internal/ai"
	"copilothub/internal/knowledge"
	"copilothub/internal/project"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const (
	aiTimeout         = 5 * time.Minute
	knowledgeFilesDir = "knowledge/files"
)

var allowedExts = map[string]string{
	".pdf":  "application/pdf",
	".md":   "text/markdown",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}

type Handler struct {
	dataDir      string // ~/.copilothub
	projectStore *project.Store
	kc           atomic.Pointer[knowledge.Client]
	aiProvider   ai.Provider
	topK         int
}

type localProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type chatTurn struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type chatReq struct {
	ProjectId  string     `json:"projectId"`
	SectionKey string     `json:"sectionKey"`
	Question   string     `json:"question"`
	History    []chatTurn `json:"history"`
	Intent     string     `json:"intent,omitempty"` // fact_lookup, as_is, to_be, relationship_query, summary
}

type chatResp struct {
	Answer         string            `json:"answer"`
	Chunks         []knowledge.Chunk `json:"chunks"`
	DetectedIntent string            `json:"detectedIntent,omitempty"`
}

func NewHandler(dataDir string, projectStore *project.Store, client *knowledge.Client, aiProvider ai.Provider, topK int) *Handler {
	if topK <= 0 {
		topK = 6
	}
	h := &Handler{dataDir: dataDir, projectStore: projectStore, aiProvider: aiProvider, topK: topK}
	if client != nil {
		h.kc.Store(client)
	}
	return h
}

// SetClient sets the knowledge client after async initialization completes.
func (h *Handler) SetClient(c *knowledge.Client) {
	h.kc.Store(c)
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects := h.registeredProjects()
	writeJSON(w, map[string]any{"projects": projects})
}

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	if h.kc.Load() == nil {
		writeError(w, "knowledge store đang khởi tạo, vui lòng thử lại sau", http.StatusServiceUnavailable)
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
	projectID := req.ProjectId
	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		intent = detectIntent(req.Question)
	}
	chunks, err := h.retrieveRelatedChunks(r.Context(), projectID, req.Question)
	if err != nil {
		writeError(w, "knowledge retrieve failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	if intent == "relationship_query" {
		chunks = h.expandByGraph(r.Context(), projectID, req.Question, chunks)
	}

	answer := h.synthesizeAnswer(r.Context(), intent, req.SectionKey, req.Question, req.History, chunks)
	writeJSON(w, chatResp{Answer: answer, Chunks: chunks, DetectedIntent: intent})
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	c := h.kc.Load()
	if c == nil {
		writeError(w, "knowledge store đang khởi tạo, vui lòng thử lại sau", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	pid := r.FormValue("projectId")
	replaceDuplicates := strings.EqualFold(strings.TrimSpace(r.FormValue("replaceDuplicates")), "true")
	destDir := h.projectFilesDir(pid)
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
			docs, _ := c.ListDocuments(r.Context(), pid)
			for _, doc := range docs {
				if doc.Name == header.Filename {
					_ = c.DeleteDocument(r.Context(), pid, doc.ID)
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

		if err := c.Ingest(r.Context(), pid, destPath, header.Filename, contentType); err != nil {
			results = append(results, uploadResult{File: header.Filename, OK: false, Message: "knowledge ingest failed"})
			continue
		}
		results = append(results, uploadResult{File: header.Filename, OK: true})
	}

	writeJSON(w, map[string]any{"results": results})
}

func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	c := h.kc.Load()
	if c == nil {
		writeJSON(w, map[string]any{"documents": []knowledge.Document{}})
		return
	}
	pid := r.URL.Query().Get("projectId")
	docs, err := c.ListDocuments(r.Context(), pid)
	if err != nil {
		writeError(w, "knowledge service unavailable: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"documents": docs})
}

func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	c := h.kc.Load()
	if c == nil {
		writeError(w, "knowledge store đang khởi tạo", http.StatusServiceUnavailable)
		return
	}
	docID := r.PathValue("id")
	pid := r.URL.Query().Get("projectId")

	// Look up the source file name before deleting metadata
	docs, _ := c.ListDocuments(r.Context(), pid)
	var sourceFile string
	for _, doc := range docs {
		if doc.ID == docID {
			sourceFile = doc.SourceFile
			break
		}
	}

	// Delete from vector DB + metadata
	if err := c.DeleteDocument(r.Context(), pid, docID); err != nil {
		writeError(w, "delete failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	// Remove the physical source file from disk
	if sourceFile != "" {
		diskPath := filepath.Join(h.projectFilesDir(pid), sourceFile)
		_ = os.Remove(diskPath)
	}

	writeJSON(w, map[string]bool{"ok": true})
}

func (h *Handler) ListPending(w http.ResponseWriter, r *http.Request) {
	if h.kc.Load() == nil {
		writeJSON(w, map[string]any{"documents": []knowledge.Document{}})
		return
	}
	pid := r.URL.Query().Get("projectId")
	docs, err := h.kc.Load().PendingDocuments(r.Context(), pid)
	if err != nil {
		writeError(w, "failed to list pending: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"documents": docs})
}

func (h *Handler) ApproveDocument(w http.ResponseWriter, r *http.Request) {
	if h.kc.Load() == nil {
		writeError(w, "knowledge service is disabled", http.StatusServiceUnavailable)
		return
	}
	docID := r.PathValue("id")
	pid := r.URL.Query().Get("projectId")
	approvedBy := r.URL.Query().Get("approvedBy")
	if approvedBy == "" {
		approvedBy = "user"
	}
	if err := h.kc.Load().ApproveDocument(r.Context(), pid, docID, approvedBy); err != nil {
		writeError(w, "approve failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (h *Handler) RejectDocument(w http.ResponseWriter, r *http.Request) {
	if h.kc.Load() == nil {
		writeError(w, "knowledge service is disabled", http.StatusServiceUnavailable)
		return
	}
	docID := r.PathValue("id")
	pid := r.URL.Query().Get("projectId")
	if err := h.kc.Load().RejectDocument(r.Context(), pid, docID); err != nil {
		writeError(w, "reject failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (h *Handler) ApproveAll(w http.ResponseWriter, r *http.Request) {
	if h.kc.Load() == nil {
		writeError(w, "knowledge service is disabled", http.StatusServiceUnavailable)
		return
	}
	pid := r.URL.Query().Get("projectId")
	approvedBy := r.URL.Query().Get("approvedBy")
	if approvedBy == "" {
		approvedBy = "user"
	}
	count, err := h.kc.Load().ApproveAllPending(r.Context(), pid, approvedBy)
	if err != nil {
		writeError(w, "approve all failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "count": count})
}

func (h *Handler) registeredProjects() []localProject {
	if h.projectStore == nil {
		return []localProject{}
	}
	projects, err := h.projectStore.List()
	if err != nil {
		return []localProject{}
	}
	result := make([]localProject, 0, len(projects))
	for _, p := range projects {
		result = append(result, localProject{ID: p.ID, Name: p.Name})
	}
	return result
}

// projectFilesDir returns the directory for storing knowledge files for a project.
// Files are stored in ~/.copilothub/projects/{projectID}/knowledge/files/
func (h *Handler) projectFilesDir(pid string) string {
	if h.projectStore != nil {
		return filepath.Join(h.projectStore.ProjectDir(pid), knowledgeFilesDir)
	}
	return filepath.Join(h.dataDir, "projects", pid, knowledgeFilesDir)
}

// maxEmbeddingChars is a safe character limit to stay within the 512-token
// embedding model constraint (~4 chars per token on average).
const maxEmbeddingChars = 1800

// splitQueryChunks splits a long query into overlapping chunks, each within
// maxEmbeddingChars, splitting at paragraph or word boundaries.
func splitQueryChunks(q string) []string {
	if len(q) <= maxEmbeddingChars {
		return []string{q}
	}
	var chunks []string
	for len(q) > 0 {
		if len(q) <= maxEmbeddingChars {
			chunks = append(chunks, q)
			break
		}
		cut := q[:maxEmbeddingChars]
		// Prefer paragraph boundary, then word boundary.
		if idx := strings.LastIndex(cut, "\n\n"); idx > maxEmbeddingChars/2 {
			cut = cut[:idx]
		} else if idx := strings.LastIndexAny(cut, "\n "); idx > maxEmbeddingChars/2 {
			cut = cut[:idx]
		}
		chunks = append(chunks, strings.TrimSpace(cut))
		q = strings.TrimSpace(q[len(cut):])
	}
	return chunks
}

func (h *Handler) retrieveRelatedChunks(ctx context.Context, projectID, question string) ([]knowledge.Chunk, error) {
	c := h.kc.Load()
	if c == nil {
		return nil, fmt.Errorf("knowledge store chưa sẵn sàng")
	}

	// For long queries (e.g. full spec documents), split into chunks and
	// retrieve for each part so we don't exceed the embedding model's limit.
	queryChunks := splitQueryChunks(question)
	var mainChunks []knowledge.Chunk
	for _, qc := range queryChunks {
		got, err := c.Retrieve(ctx, projectID, qc, h.topK)
		if err != nil {
			return nil, err
		}
		mainChunks = append(mainChunks, got...)
	}

	// Extract key terms for broader retrieval
	words := strings.Fields(question)
	var relatedChunks []knowledge.Chunk
	if len(words) > 2 {
		subQuery := strings.Join(words[:len(words)/2], " ")
		if len(subQuery) > maxEmbeddingChars {
			subQuery = subQuery[:maxEmbeddingChars]
		}
		if sub, err := c.Retrieve(ctx, projectID, subQuery, h.topK/2); err == nil {
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

func detectIntent(question string) string {
	q := strings.ToLower(question)
	if strings.Contains(q, "quan hệ") || strings.Contains(q, "liên quan") || strings.Contains(q, "ảnh hưởng") || strings.Contains(q, "phụ thuộc") {
		return "relationship_query"
	}
	if strings.Contains(q, "hiện tại") || strings.Contains(q, "đang") || strings.Contains(q, "as-is") {
		return "as_is"
	}
	if strings.Contains(q, "thay đổi") || strings.Contains(q, "to-be") || strings.Contains(q, "impact") || strings.Contains(q, "mới") {
		return "to_be"
	}
	if strings.Contains(q, "tóm tắt") || strings.Contains(q, "summary") {
		return "summary"
	}
	return "fact_lookup"
}

func (h *Handler) expandByGraph(ctx context.Context, projectID, question string, chunks []knowledge.Chunk) []knowledge.Chunk {
	nodes, err := h.kc.Load().SearchGraphNodes(ctx, projectID, question)
	if err != nil || len(nodes) == 0 {
		return chunks
	}
	for _, node := range nodes {
		neighbors, edges, err := h.kc.Load().GetGraphNeighbors(ctx, projectID, node.ID, 1)
		if err != nil {
			continue
		}
		for _, n := range neighbors {
			chunks = append(chunks, knowledge.Chunk{
				Content: fmt.Sprintf("[Graph] %s (%s)", n.CanonicalName, n.Type),
				Score:   0.7,
			})
		}
		for _, e := range edges {
			chunks = append(chunks, knowledge.Chunk{
				Content: fmt.Sprintf("[Relation] %s", e.RelationType),
				Score:   0.6,
			})
		}
		if len(chunks) > h.topK*3 {
			break
		}
	}
	return chunks
}

func (h *Handler) synthesizeAnswer(ctx context.Context, intent, sectionKey, question string, history []chatTurn, chunks []knowledge.Chunk) string {
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

	intentGuide := map[string]string{
		"relationship_query": "Tập trung giải thích các quan hệ phụ thuộc, tác động qua lại và chuỗi ảnh hưởng giữa các thực thể nghiệp vụ.",
		"as_is":              "Phân tích trạng thái hiện tại (As-is), không giả định thay đổi mới.",
		"to_be":              "Phân tích thay đổi To-be và impact so với luồng hiện tại.",
		"summary":            "Tóm tắt ngắn gọn, rõ ràng theo ý chính.",
		"fact_lookup":        "Trả lời trực tiếp theo facts trong knowledge đã verify.",
	}
	guide := intentGuide[intent]
	if guide == "" {
		guide = intentGuide["fact_lookup"]
	}

	prompt := fmt.Sprintf(`%s
# Câu hỏi hiện tại
%s

# Intent phát hiện
%s

# Yêu cầu trả lời
Bạn là Business Analyst và viết theo phong cách NotebookLM: tự nhiên, mạch lạc, dễ đọc, tổng hợp từ nhiều nguồn nhưng không sa đà kỹ thuật.

BẮT BUỘC format output dưới dạng MARKDOWN có cấu trúc rõ ràng:
- Dùng heading cấp 2 (## Tiêu đề) cho từng phần lớn.
- Dùng numbered list cho các bước tuần tự, bullet list cho các ý rời.
- Dùng checkbox “- [ ]” cho checklist xác nhận.
- In đậm (**từ khóa**) cho các khái niệm nghiệp vụ quan trọng.
- Không viết plain text liền mạch không có heading hay list.

Chỉ sử dụng dữ liệu từ knowledge đã được duyệt.
Nếu dữ liệu chưa đủ, phải có mục “## Phần còn thiếu dữ liệu” và liệt kê câu hỏi cần bổ sung.
`, contextBuilder.String(), question, guide)

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

func (h *Handler) GetDocumentContent(w http.ResponseWriter, r *http.Request) {
	c := h.kc.Load()
	if c == nil {
		writeError(w, "knowledge store not ready", http.StatusServiceUnavailable)
		return
	}
	docID := r.URL.Query().Get("docId")
	pid := r.URL.Query().Get("projectId")
	ctx := r.Context()

	allDocs, _ := c.ListDocuments(ctx, pid)
	pendingDocs, _ := c.PendingDocuments(ctx, pid)

	var sourceFile, name string
	for _, d := range append(allDocs, pendingDocs...) {
		if d.ID == docID {
			sourceFile = d.SourceFile
			name = d.Name
			break
		}
	}
	if sourceFile == "" {
		writeError(w, "document not found", http.StatusNotFound)
		return
	}

	fp := filepath.Join(h.projectFilesDir(pid), sourceFile)
	content, err := knowledge.ReadFileContent(fp, sourceFile)
	if err != nil {
		writeError(w, "cannot read file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"content":    content,
		"name":       name,
		"sourceFile": sourceFile,
		"isMarkdown": strings.HasSuffix(strings.ToLower(sourceFile), ".md"),
	})
}
