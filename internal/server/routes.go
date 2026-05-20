package server

import (
	"copilothub/internal/ai"
	"copilothub/internal/config"
	"copilothub/internal/features/specclarify"
	"copilothub/internal/features/wiki"
	"copilothub/internal/handler"
	"copilothub/internal/hub"
	"copilothub/internal/knowledge"
	"encoding/json"
	"fmt"
	"net/http"
)

func setupRoutes(mux *http.ServeMux, repoPath string) {
	cfgStore := config.NewStore(repoPath)
	cfg, _ := cfgStore.Load()

	fmt.Printf("Using AI provider: %s, model: %s\n", cfg.AI.Provider, cfg.AI.Model)

	aiProvider := ai.NewProvider(cfg.AI.Provider, cfg.AI.Token, cfg.AI.Model, cfg.AI.BaseURL, repoPath)

	// Build registry with built-in features
	registry := hub.NewRegistry()
	registry.Register(specclarify.New())
	registry.Register(wiki.New())

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

	// Embedding model download progress
	mux.HandleFunc("GET /api/embedding/check", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(knowledge.EmbedProgress.Get()) //nolint:errcheck
	})

	mux.HandleFunc("GET /api/embedding/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		send := func(p knowledge.ModelProgress) {
			data, _ := json.Marshal(p)
			fmt.Fprintf(w, "data: %s\n\n", data) //nolint:errcheck
			flusher.Flush()
		}

		p := knowledge.EmbedProgress.Get()
		send(p)
		if p.State == knowledge.ModelStateReady || p.State == knowledge.ModelStateError {
			return
		}

		ch := knowledge.EmbedProgress.Subscribe()
		defer knowledge.EmbedProgress.Unsubscribe(ch)

		for {
			select {
			case <-r.Context().Done():
				return
			case prog, ok := <-ch:
				if !ok {
					return
				}
				send(prog)
				if prog.State == knowledge.ModelStateReady || prog.State == knowledge.ModelStateError {
					return
				}
			}
		}
	})
}
