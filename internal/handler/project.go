package handler

import (
	"copilothub/internal/project"
	"copilothub/internal/repo"
	"encoding/json"
	"net/http"
)

type ProjectHandler struct {
	store *project.Store
}

func NewProjectHandler(store *project.Store) *ProjectHandler {
	return &ProjectHandler{store: store}
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
