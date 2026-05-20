package hub

import (
	"copilothub/internal/ai"
	"copilothub/internal/config"
	"copilothub/internal/project"
	"net/http"
)

type Feature interface {
	ID() string
	Manifest() Manifest
	Init(ctx FeatureContext) error
	RegisterRoutes(mux *http.ServeMux)
}

type FeatureContext struct {
	WorkDir      string // kept for backward compat (may be empty)
	DataDir      string // ~/.copilothub — central data directory
	AIProvider   ai.Provider
	Config       *config.Store
	ProjectStore *project.Store
}

type Manifest struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Description   string `json:"description"`
	Icon          string `json:"icon"`
	Category      string `json:"category"`
	Author        string `json:"author"`
	Type          string `json:"type"` // "builtin" | "external"
	FrontendRoute string `json:"frontendRoute"`
}
