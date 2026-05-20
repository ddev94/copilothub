package specclarify

import (
	"copilothub/internal/hub"
	"copilothub/pkg/version"
	"net/http"
)

type Feature struct {
	h *Handler
}

func New() *Feature { return &Feature{} }

func (f *Feature) ID() string { return "spec-clarify" }

func (f *Feature) Manifest() hub.Manifest {
	return hub.Manifest{
		ID:            "spec-clarify",
		Name:          "Spec Clarify",
		Version:       version.Version,
		Description:   "Analyze and clarify spec documents against source code or wiki",
		Icon:          "🔍",
		Category:      "documentation",
		Author:        "aikit",
		Type:          "builtin",
		FrontendRoute: "/features/spec-clarify",
	}
}

func (f *Feature) Init(ctx hub.FeatureContext) error {
	f.h = NewHandler(ctx.AIProvider, ctx.WorkDir)
	return nil
}

func (f *Feature) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /clarify", f.h.Clarify)
	mux.HandleFunc("POST /refine", f.h.Refine)
}
