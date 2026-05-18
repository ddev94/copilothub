package server

import (
	"copilothub/internal/ai"
	"copilothub/internal/config"
	"copilothub/internal/features/specdesigner"
	"copilothub/internal/handler"
	"copilothub/internal/hub"
	"encoding/json"
	"fmt"
	"net/http"
)

func setupRoutes(mux *http.ServeMux, repoPath string) {
	cfgStore := config.NewStore(repoPath)
	cfg, _ := cfgStore.Load()

	fmt.Printf("Using AI model: %s\n", cfg.AI.Model)

	aiProvider := ai.NewSDKProvider(cfg.AI.Token, cfg.AI.Model, repoPath)

	// Build registry with built-in features
	registry := hub.NewRegistry()
	registry.Register(specdesigner.New())

	// Load and register external plugins
	pluginReg, _ := hub.LoadPluginRegistry()
	for _, p := range pluginReg.Plugins {
		registry.Register(hub.NewExternalFeature(p))
	}

	// Mount all feature routes under /api/features/{id}/
	registry.RegisterRoutes(mux, hub.FeatureContext{
		WorkDir:    repoPath,
		AIProvider: aiProvider,
		Config:     cfgStore,
	})

	// Hub-level routes
	hubH := handler.NewHubHandler(registry)
	repoH := handler.NewRepoHandler(repoPath)
	cfgH := handler.NewConfigHandler(repoPath)

	mux.HandleFunc("GET /api/hub/features", hubH.ListFeatures)
	mux.HandleFunc("GET /api/repo", repoH.Info)
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
