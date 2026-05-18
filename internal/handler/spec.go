package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"spec-designer/internal/spec"
)

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
