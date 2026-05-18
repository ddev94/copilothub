package handler

import (
	"net/http"
	"aikit/internal/repo"
)

type RepoHandler struct {
	scanner *repo.Scanner
}

func NewRepoHandler(repoPath string) *RepoHandler {
	return &RepoHandler{scanner: repo.NewScanner(repoPath)}
}

func (h *RepoHandler) Info(w http.ResponseWriter, r *http.Request) {
	info, err := h.scanner.Scan()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, info)
}
