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

	// Create handler — uses project store for listing and DataDir for knowledge storage
	f.h = NewHandler(ctx.DataDir, ctx.ProjectStore, nil, ctx.AIProvider, cfg.Knowledge.TopK)

	if cfg.Knowledge.Enabled {
		storeDir := filepath.Join(ctx.DataDir, "knowledge-store")
		embedCfg := knowledge.EmbeddingConfig{
			Provider: cfg.Knowledge.EmbeddingProvider,
			Model:    cfg.Knowledge.EmbeddingModel,
			Key:      cfg.Knowledge.EmbeddingKey,
			URL:      cfg.Knowledge.EmbeddingURL,
		}
		go func() {
			client, err := knowledge.NewClient(storeDir, embedCfg)
			if err != nil {
				fmt.Printf("[wiki] knowledge store failed: %v\n", err)
				return
			}
			f.h.SetClient(client)
			fmt.Println("[wiki] knowledge store ready")
		}()
	}

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
	mux.HandleFunc("GET /knowledge/content", f.h.GetDocumentContent)
}
