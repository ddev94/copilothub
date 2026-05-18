package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"spec-designer/internal/ai"
	"spec-designer/internal/config"
	"spec-designer/internal/handler"
)

func setupRoutes(mux *http.ServeMux, repoPath string) {
	cfgStore := config.NewStore(repoPath)
	cfg, _ := cfgStore.Load()

	fmt.Printf("Using AI model: %s\n", cfg.AI.Model)

	aiProvider := ai.NewSDKProvider(cfg.AI.Token, cfg.AI.Model, repoPath)

	repoH := handler.NewRepoHandler(repoPath)
	specH := handler.NewSpecHandler(repoPath)
	aiH := handler.NewAIHandler(aiProvider, repoPath)
	cfgH := handler.NewConfigHandler(repoPath)

	mux.HandleFunc("GET /api/repo", repoH.Info)

	// Multi-spec CRUD
	mux.HandleFunc("GET /api/specs", specH.List)
	mux.HandleFunc("POST /api/specs", specH.Create)
	mux.HandleFunc("GET /api/spec/{id}", specH.Get)
	mux.HandleFunc("PUT /api/spec/{id}", specH.Save)
	mux.HandleFunc("DELETE /api/spec/{id}", specH.Delete)

	mux.HandleFunc("POST /api/ai/suggest", aiH.Suggest)
	mux.HandleFunc("POST /api/ai/clarify", aiH.Clarify)
	mux.HandleFunc("POST /api/ai/generate-spec", aiH.GenerateSpec)
	mux.HandleFunc("GET /api/config", cfgH.Get)
	mux.HandleFunc("PUT /api/config", cfgH.Save)

	mux.HandleFunc("GET /api/auth/status", func(w http.ResponseWriter, r *http.Request) {
		cliPath := ai.FindCLI()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"cliFound": cliPath != "",
			"cliPath":  cliPath,
		})
	})
}
