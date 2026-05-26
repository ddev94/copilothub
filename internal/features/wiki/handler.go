package wiki

import (
	"context"
	"copilothub/internal/ai"
	"copilothub/internal/config"
	"copilothub/internal/knowledge"
	"copilothub/internal/project"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	stepDetectIntent     = "detect_intent"
	stepRetrieveChunks   = "retrieve_chunks"
	stepExpandGraph      = "expand_graph"
	stepSynthesizeAnswer = "synthesize_answer"
)

type chatStepEvent struct {
	Step    string         `json:"step"`
	Status  string         `json:"status"`
	Summary string         `json:"summary"`
	Data    map[string]any `json:"data,omitempty"`
}

func writeSSE(w http.ResponseWriter, event string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func (h *Handler) ChatStream(w http.ResponseWriter, r *http.Request) {
	c := h.getKC()
	if c == nil {
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

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	intent := strings.TrimSpace(req.Intent)
	_ = writeSSE(w, "step", chatStepEvent{Step: stepDetectIntent, Status: "started", Summary: "Đang xác định intent"})
	if intent == "" {
		intent = h.detectIntentAI(r.Context(), req.Question)
	}
	queryType := detectQueryType(req.Question)
	_ = writeSSE(w, "step", chatStepEvent{Step: stepDetectIntent, Status: "completed", Summary: "Đã xác định intent", Data: map[string]any{
		"detectedIntent": intent,
		"queryType":      queryType,
	}})

	out, err := h.runChatWithReporter(r.Context(), c, chatInput{
		ProjectID:  req.ProjectId,
		SectionKey: req.SectionKey,
		Question:   req.Question,
		History:    req.History,
		Intent:     intent,
	}, func(step string, status string, summary string, data map[string]any) {
		_ = writeSSE(w, "step", chatStepEvent{Step: step, Status: status, Summary: summary, Data: data})
	})
	if err != nil {
		_ = writeSSE(w, "error", map[string]string{"error": "knowledge retrieve failed: " + err.Error()})
		return
	}

	// Final telemetry
	_ = writeSSE(w, "step", chatStepEvent{Step: stepSynthesizeAnswer, Status: "completed", Summary: "Đã tổng hợp câu trả lời", Data: map[string]any{
		"queryType":       out.QueryType,
		"stopReason":      out.StopReason,
		"usedGraph":       out.UsedGraph,
		"chunksReturned":  len(out.Chunks),
		"sourceDiversity": countUniqueSources(out.Chunks),
	}})

	_ = writeSSE(w, "final", chatResp{Answer: out.Answer, Chunks: out.Chunks, DetectedIntent: out.DetectedIntent})
}

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
	cfgStore     *config.Store
	topK         int

	kcMu  sync.Mutex
	kc    atomic.Pointer[knowledge.Client]
	kcKey string // fingerprint of last-used embedding config
}

// newAI creates a fresh AI provider from the current config (cheap, per-request).
func (h *Handler) newAI() ai.Provider {
	cfg, _ := h.cfgStore.Load()
	return ai.NewProvider(cfg.AI.Provider, cfg.AI.Token, cfg.AI.Model, cfg.AI.BaseURL, h.dataDir)
}

// getKC returns the cached knowledge client, re-creating it if the embedding
// config has changed since last init. Init is lazy — only triggered on first use.
func (h *Handler) getKC() *knowledge.Client {
	cfg, _ := h.cfgStore.Load()
	key := cfg.Knowledge.EmbeddingProvider + "|" + cfg.Knowledge.EmbeddingModel + "|" +
		cfg.Knowledge.EmbeddingKey + "|" + cfg.Knowledge.EmbeddingURL

	if c := h.kc.Load(); c != nil && h.kcKey == key {
		return c
	}

	h.kcMu.Lock()
	defer h.kcMu.Unlock()
	// double-check after acquiring lock
	if c := h.kc.Load(); c != nil && h.kcKey == key {
		return c
	}

	if !cfg.Knowledge.Enabled {
		return nil
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
		fmt.Printf("[wiki] knowledge store init failed: %v\n", err)
		return nil
	}
	h.kc.Store(client)
	h.kcKey = key
	return client
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

func NewHandler(dataDir string, projectStore *project.Store, cfgStore *config.Store, topK int) *Handler {
	if topK <= 0 {
		topK = 15
	}
	return &Handler{dataDir: dataDir, projectStore: projectStore, cfgStore: cfgStore, topK: topK}
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects := h.registeredProjects()
	writeJSON(w, map[string]any{"projects": projects})
}

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	c := h.getKC()
	if c == nil {
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
	out, err := h.runChat(r.Context(), c, chatInput{
		ProjectID:  req.ProjectId,
		SectionKey: req.SectionKey,
		Question:   req.Question,
		History:    req.History,
		Intent:     req.Intent,
	})
	if err != nil {
		writeError(w, "knowledge retrieve failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, chatResp{Answer: out.Answer, Chunks: out.Chunks, DetectedIntent: out.DetectedIntent})
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	c := h.getKC()
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

		// For PDFs: use the full pipeline (async refine → chunk → embed → graph)
		// For other files: use standard ingest (chunk → embed → graph)
		ingestPath := destPath
		ingestName := header.Filename
		if ext == ".pdf" {
			refineFn := func(ctx context.Context, filePath, fileName string) string {
				return h.refinePDF(ctx, filePath, fileName)
			}
			if _, err := c.IngestPipelineAsync(r.Context(), pid, destPath, header.Filename, contentType, refineFn); err != nil {
				results = append(results, uploadResult{File: header.Filename, OK: false, Message: "knowledge ingest failed: " + err.Error()})
				continue
			}
		} else {
			if _, err := c.IngestAsync(r.Context(), pid, ingestPath, ingestName, contentType); err != nil {
				results = append(results, uploadResult{File: header.Filename, OK: false, Message: "knowledge ingest failed: " + err.Error()})
				continue
			}
		}
		results = append(results, uploadResult{File: header.Filename, OK: true, Message: "processing in background"})
	}

	writeJSON(w, map[string]any{"results": results})
}

func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	c := h.getKC()
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
	c := h.getKC()
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

	// Remove the physical source file and any associated refined file from disk
	if sourceFile != "" {
		filesDir := h.projectFilesDir(pid)
		diskPath := filepath.Join(filesDir, sourceFile)
		_ = os.Remove(diskPath)

		// Also remove refined markdown if it exists (for PDFs)
		if strings.HasSuffix(strings.ToLower(sourceFile), ".pdf") {
			_ = os.Remove(diskPath + ".refined.md")
		}
		// If sourceFile itself is a .refined.md, also remove the original PDF
		if strings.HasSuffix(sourceFile, ".refined.md") {
			origPDF := strings.TrimSuffix(sourceFile, ".refined.md")
			_ = os.Remove(filepath.Join(filesDir, origPDF))
		}
	}

	writeJSON(w, map[string]bool{"ok": true})
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

func detectIntent(question string) string {
	q := strings.ToLower(question)
	// Only classify away from fact_lookup with EXTREMELY explicit signals.
	// "quan hệ giữa X và Y" requires both "quan hệ giữa" AND "và"
	if (strings.Contains(q, "quan hệ giữa") || strings.Contains(q, "phụ thuộc giữa")) && strings.Contains(q, "và") {
		return "relationship_query"
	}
	// Only as_is when explicitly asking "as-is" or "trạng thái hiện tại" (not just "hiện tại")
	if strings.Contains(q, "as-is") || strings.Contains(q, "trạng thái hiện tại") {
		return "as_is"
	}
	// Only to_be when explicitly asking "to-be" or comparing before/after
	if strings.Contains(q, "to-be") || (strings.Contains(q, "so sánh") && strings.Contains(q, "hiện tại")) {
		return "to_be"
	}
	if strings.Contains(q, "tóm tắt") || strings.Contains(q, "tổng quan") || strings.Contains(q, "summary") {
		return "summary"
	}
	// Default: always fact_lookup
	return "fact_lookup"
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
	c := h.getKC()
	if c == nil {
		writeError(w, "knowledge store not ready", http.StatusServiceUnavailable)
		return
	}
	docID := r.URL.Query().Get("docId")
	pid := r.URL.Query().Get("projectId")
	ctx := r.Context()

	allDocs, _ := c.ListDocuments(ctx, pid)

	var sourceFile, name string
	for _, d := range allDocs {
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

func (h *Handler) IngestProgress(w http.ResponseWriter, r *http.Request) {
	progress := knowledge.IngestProgressTracker.Get()
	writeJSON(w, progress)
}

func (h *Handler) ResyncOrphans(w http.ResponseWriter, r *http.Request) {
	c := h.kc.Load()
	if c == nil {
		writeError(w, "knowledge store not ready", http.StatusServiceUnavailable)
		return
	}
	pid := r.URL.Query().Get("projectId")
	if pid == "" {
		writeError(w, "projectId required", http.StatusBadRequest)
		return
	}
	filesDir := h.projectFilesDir(pid)
	count := c.SyncOrphanFiles(pid, filesDir)
	writeJSON(w, map[string]any{"ok": true, "resynced": count})
}

func (h *Handler) GraphStats(w http.ResponseWriter, r *http.Request) {
	c := h.kc.Load()
	if c == nil {
		writeError(w, "knowledge store not ready", http.StatusServiceUnavailable)
		return
	}
	health, err := c.GraphHealth()
	if err != nil {
		writeError(w, "graph stats unavailable: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{
		"nodes":        health.NodeCount,
		"edges":        health.EdgeCount,
		"revision":     health.Revision,
		"lastModified": health.LastModified,
	})
}

func (h *Handler) RebuildGraph(w http.ResponseWriter, r *http.Request) {
	c := h.getKC()
	if c == nil {
		writeError(w, "knowledge store not ready", http.StatusServiceUnavailable)
		return
	}
	pid := r.URL.Query().Get("projectId")
	if pid == "" {
		writeError(w, "projectId is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	if err := c.RebuildGraphForProject(ctx, pid); err != nil {
		writeError(w, "graph rebuild failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	health, _ := c.GraphHealth()
	writeJSON(w, map[string]any{
		"ok":       true,
		"nodes":    health.NodeCount,
		"edges":    health.EdgeCount,
		"revision": health.Revision,
	})
}
