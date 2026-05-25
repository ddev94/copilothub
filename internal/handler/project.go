package handler

import (
	"copilothub/internal/config"
	"copilothub/internal/knowledge"
	"copilothub/internal/project"
	"copilothub/internal/repo"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
)

type ProjectHandler struct {
	store    *project.Store
	cfgStore *config.Store
	dataDir  string

	kcMu  sync.Mutex
	kc    atomic.Pointer[knowledge.Client]
	kcKey string
}

func NewProjectHandler(store *project.Store, cfgStore *config.Store, dataDir string) *ProjectHandler {
	return &ProjectHandler{store: store, cfgStore: cfgStore, dataDir: dataDir}
}

// getKC returns a lazily-initialized knowledge client, recreated if embedding config changes.
func (h *ProjectHandler) getKC() *knowledge.Client {
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
		fmt.Printf("[project] knowledge store init failed: %v\n", err)
		return nil
	}
	h.kc.Store(client)
	h.kcKey = key
	return client
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.store.List()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"projects": projects})
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.store.Get(id)
	if err != nil {
		writeError(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, p)
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		writeError(w, "name is required", http.StatusBadRequest)
		return
	}
	p, err := h.store.Create(req.Name)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, p)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.Delete(id); err != nil {
		writeError(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.store.Get(id)
	if err != nil {
		writeError(w, err.Error(), http.StatusNotFound)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name != "" {
		p.Name = req.Name
	}
	if err := h.store.Update(p); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, p)
}

// AddRepo clones and registers a new repository for a project.
func (h *ProjectHandler) AddRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		RepoURL string `json:"repoURL"`
		Branch  string `json:"branch"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.RepoURL == "" {
		writeError(w, "repoURL is required", http.StatusBadRequest)
		return
	}
	repo, err := h.store.AddRepo(id, req.RepoURL, req.Branch, req.Name)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, repo)
}

// RemoveRepo disconnects and removes a repository from a project.
func (h *ProjectHandler) RemoveRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repoID := r.PathValue("repoId")
	if err := h.store.RemoveRepo(id, repoID); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// ChangeRepoBranch re-clones a repository on a different branch.
func (h *ProjectHandler) ChangeRepoBranch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repoID := r.PathValue("repoId")
	var req struct {
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.store.ChangeRepoBranch(id, repoID, req.Branch); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p, _ := h.store.Get(id)
	writeJSON(w, p)
}

// RepoInfo returns scanner info for the first connected repository (or a specific one).
func (h *ProjectHandler) RepoInfo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.store.Get(id)
	if err != nil {
		writeError(w, err.Error(), http.StatusNotFound)
		return
	}
	if len(p.Repositories) == 0 {
		writeError(w, "no repository connected", http.StatusBadRequest)
		return
	}

	repoID := r.URL.Query().Get("repoId")
	if repoID == "" {
		repoID = p.Repositories[0].ID
	}

	srcDir := h.store.RepoSourceDir(id, repoID)
	scanner := repo.NewScanner(srcDir)
	info, err := scanner.Scan()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, info)
}

// ── Deep Repo Indexing ───────────────────────────────────────────────

// IndexRepo triggers async code embedding for a repository.
func (h *ProjectHandler) IndexRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repoID := r.PathValue("repoId")

	p, err := h.store.Get(id)
	if err != nil {
		writeError(w, err.Error(), http.StatusNotFound)
		return
	}

	var target *project.Repository
	for i := range p.Repositories {
		if p.Repositories[i].ID == repoID {
			target = &p.Repositories[i]
			break
		}
	}
	if target == nil || !target.RepoCloned {
		writeError(w, "repository not found or not cloned", http.StatusBadRequest)
		return
	}

	kc := h.getKC()
	if kc == nil {
		writeError(w, "knowledge store not available — check embedding config", http.StatusServiceUnavailable)
		return
	}

	repoDir := h.store.RepoSourceDir(id, repoID)
	kc.IndexRepoAsync(id, repoID, repoDir)

	writeJSON(w, map[string]string{"status": "indexing"})
}

// IndexRepoStatus returns the indexing status for a specific repository.
func (h *ProjectHandler) IndexRepoStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repoID := r.PathValue("repoId")

	kc := h.getKC()
	if kc == nil {
		writeJSON(w, knowledge.RepoIndexStatus{State: "none", Message: "knowledge store not configured"})
		return
	}

	writeJSON(w, kc.GetRepoIndexStatus(id, repoID))
}

// IndexRepoStream streams repo indexing progress via SSE.
func (h *ProjectHandler) IndexRepoStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	send := func(p knowledge.RepoIndexProgress) {
		data, _ := json.Marshal(p)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Send current state
	p := knowledge.RepoIndexTracker.Get()
	send(p)
	if p.State == "indexed" || p.State == "error" || p.State == "" {
		return
	}

	ch := knowledge.RepoIndexTracker.Subscribe()
	defer knowledge.RepoIndexTracker.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case prog, ok := <-ch:
			if !ok {
				return
			}
			send(prog)
			if prog.State == "indexed" || prog.State == "error" {
				return
			}
		}
	}
}

// DeleteRepoIndex removes the code index for a repository.
func (h *ProjectHandler) DeleteRepoIndex(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repoID := r.PathValue("repoId")

	kc := h.getKC()
	if kc == nil {
		writeError(w, "knowledge store not available", http.StatusServiceUnavailable)
		return
	}

	if err := kc.DeleteRepoIndex(r.Context(), id, repoID); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// ConnectRepo is kept for backward compatibility.
func (h *ProjectHandler) ConnectRepo(w http.ResponseWriter, r *http.Request) {
	h.AddRepo(w, r)
}

// DisconnectRepo is kept for backward compatibility — removes all repos.
func (h *ProjectHandler) DisconnectRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DisconnectRepo(id); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p, _ := h.store.Get(id)
	writeJSON(w, p)
}

// ChangeBranch is kept for backward compatibility — changes branch of first repo.
func (h *ProjectHandler) ChangeBranch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.store.Get(id)
	if err != nil {
		writeError(w, err.Error(), http.StatusNotFound)
		return
	}
	if len(p.Repositories) == 0 {
		writeError(w, "no repository connected", http.StatusBadRequest)
		return
	}
	var req struct {
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.store.ChangeRepoBranch(id, p.Repositories[0].ID, req.Branch); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p, _ = h.store.Get(id)
	writeJSON(w, p)
}
