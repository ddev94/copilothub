package specdesigner

import (
	"copilothub/internal/hub"
	"copilothub/pkg/version"
	"net/http"
)

type Feature struct {
	specH *SpecHandler
	aiH   *AIHandler
}

func New() *Feature { return &Feature{} }

func (f *Feature) ID() string { return "spec-designer" }

func (f *Feature) Manifest() hub.Manifest {
	return hub.Manifest{
		ID:            "spec-designer",
		Name:          "Spec Designer",
		Version:       version.Version,
		Description:   "Generate SRD documents and user stories from requirements",
		Icon:          "📄",
		Category:      "documentation",
		Author:        "aikit",
		Type:          "builtin",
		FrontendRoute: "/features/spec-designer",
	}
}

func (f *Feature) Init(ctx hub.FeatureContext) error {
	f.specH = NewSpecHandler(ctx.WorkDir)
	f.aiH = NewAIHandler(ctx.AIProvider, ctx.WorkDir)
	return nil
}

func (f *Feature) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /specs", f.specH.List)
	mux.HandleFunc("POST /specs", f.specH.Create)
	mux.HandleFunc("GET /spec/{id}", f.specH.Get)
	mux.HandleFunc("PUT /spec/{id}", f.specH.Save)
	mux.HandleFunc("DELETE /spec/{id}", f.specH.Delete)
	mux.HandleFunc("POST /ai/suggest", f.aiH.Suggest)
	mux.HandleFunc("POST /ai/clarify", f.aiH.Clarify)
	mux.HandleFunc("POST /ai/generate-spec", f.aiH.GenerateSpec)
}
