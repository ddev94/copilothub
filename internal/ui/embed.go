package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

func Handler() http.Handler {
	sub, _ := fs.Sub(dist, "dist")
	return &spaHandler{fs: sub}
}

type spaHandler struct {
	fs fs.FS
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := h.fs.Open(path); err != nil {
		// SPA fallback: serve index.html for unknown paths
		data, err := fs.ReadFile(h.fs, "index.html")
		if err != nil {
			http.Error(w, "frontend not built — run: make build-frontend", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data) //nolint:errcheck
		return
	}
	http.FileServer(http.FS(h.fs)).ServeHTTP(w, r)
}
