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
	h *Handler
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
		storeDir := filepath.Join(ctx.WorkDir, ".spec-designer", "knowledge-store")
		client, err := knowledge.NewClient(storeDir)
		if err != nil {
			fmt.Printf("[knowledge] store failed to initialise: %v — knowledge disabled\n", err)
		} else {
			knowledgeClient = client
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
	mux.HandleFunc("GET /knowledge/pending", f.h.ListPending)
	mux.HandleFunc("POST /knowledge/document/{id}/approve", f.h.ApproveDocument)
	mux.HandleFunc("POST /knowledge/document/{id}/reject", f.h.RejectDocument)
	mux.HandleFunc("POST /knowledge/approve-all", f.h.ApproveAll)
}
