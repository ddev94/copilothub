package specdesigner

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"copilothub/internal/knowledge"
)

const knowledgeFilesDir = ".spec-designer/knowledge/files"

var allowedExts = map[string]string{
	".pdf":  "application/pdf",
	".md":   "text/markdown",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}

type KnowledgeHandler struct {
	repoPath string
	client   *knowledge.Client
}

func NewKnowledgeHandler(repoPath string, client *knowledge.Client) *KnowledgeHandler {
	return &KnowledgeHandler{repoPath: repoPath, client: client}
}

func (h *KnowledgeHandler) projectID() string {
	sum := sha256.Sum256([]byte(h.repoPath))
	return fmt.Sprintf("%x", sum[:8])
}

func (h *KnowledgeHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, "knowledge service is disabled", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, "file field required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType, ok := allowedExts[ext]
	if !ok {
		writeError(w, "unsupported file type; allowed: .pdf, .md, .docx", http.StatusBadRequest)
		return
	}

	destDir := filepath.Join(h.repoPath, knowledgeFilesDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		writeError(w, "failed to create storage dir: "+err.Error(), http.StatusInternalServerError)
		return
	}

	destPath := filepath.Join(destDir, header.Filename)
	dst, err := os.Create(destPath)
	if err != nil {
		writeError(w, "failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		writeError(w, "failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.client.Ingest(r.Context(), h.projectID(), destPath, header.Filename, contentType); err != nil {
		writeError(w, "knowledge ingest failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	writeJSON(w, map[string]string{"ok": "true", "file": header.Filename})
}

func (h *KnowledgeHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeJSON(w, map[string]any{"documents": []knowledge.Document{}})
		return
	}
	docs, err := h.client.ListDocuments(r.Context(), h.projectID())
	if err != nil {
		writeError(w, "knowledge service unavailable: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"documents": docs})
}

func (h *KnowledgeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, "knowledge service is disabled", http.StatusServiceUnavailable)
		return
	}
	docID := r.PathValue("id")
	if err := h.client.DeleteDocument(r.Context(), h.projectID(), docID); err != nil {
		writeError(w, "delete failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
