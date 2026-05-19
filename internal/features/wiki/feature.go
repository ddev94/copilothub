package wiki

import (
	"copilothub/internal/hub"
	"copilothub/internal/knowledge"
	"copilothub/pkg/version"
	"fmt"
	"net/http"
	"path/filepath"
)

type Feature struct {
	h       *Handler
	sidecar *knowledge.Sidecar
}

func New() *Feature { return &Feature{} }

func (f *Feature) ID() string { return "wiki" }

func (f *Feature) Manifest() hub.Manifest {
	return hub.Manifest{
		ID:            "wiki",
		Name:          "Wiki",
		Version:       version.Version,
		Description:   "Chat and manage project knowledge across local projects",
		Icon:          "📚",
		Category:      "knowledge",
		Author:        "aikit",
		Type:          "builtin",
		FrontendRoute: "/features/wiki",
	}
}

func (f *Feature) Init(ctx hub.FeatureContext) error {
	cfg, _ := ctx.Config.Load()

	var knowledgeClient *knowledge.Client
	if cfg.Knowledge.Enabled {
		chromaDir := filepath.Join(ctx.WorkDir, ".spec-designer", "chroma")
		sidecar, url, err := knowledge.StartSidecar(chromaDir)
		if err != nil {
			fmt.Printf("[knowledge] sidecar failed to start: %v — knowledge disabled\n", err)
		} else if url != "" {
			f.sidecar = sidecar
			knowledgeClient = knowledge.NewClient(url)
		}
	}

	f.h = NewHandler(ctx.WorkDir, knowledgeClient, ctx.AIProvider, cfg.Knowledge.TopK)
	return nil
}

func (f *Feature) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /projects", f.h.ListProjects)
	mux.HandleFunc("POST /chat", f.h.Chat)
	mux.HandleFunc("POST /knowledge/upload", f.h.Upload)
	mux.HandleFunc("GET /knowledge/documents", f.h.ListDocuments)
	mux.HandleFunc("DELETE /knowledge/document/{id}", f.h.DeleteDocument)
}

func (f *Feature) Stop() {
	if f.sidecar != nil {
		f.sidecar.Stop()
	}
}
