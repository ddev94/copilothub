package server

import (
	"copilothub/internal/ai"
	"copilothub/internal/config"
	"copilothub/internal/features/specclarify"
	"copilothub/internal/features/wiki"
	"copilothub/internal/handler"
	"copilothub/internal/hub"
	"copilothub/internal/knowledge"
	"copilothub/internal/project"
	"encoding/json"
	"fmt"
	"net/http"
)

func setupRoutes(mux *http.ServeMux, dataDir string) {
	cfgStore := config.NewStore(dataDir)
	cfg, _ := cfgStore.Load()
	projStore := project.NewStore(dataDir)

	aiProvider := ai.NewProvider(cfg.AI.Provider, cfg.AI.Token, cfg.AI.Model, cfg.AI.BaseURL, dataDir)

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
		DataDir:      dataDir,
		AIProvider:   aiProvider,
		Config:       cfgStore,
		ProjectStore: projStore,
	})

	// Hub-level routes
	hubH := handler.NewHubHandler(registry)
	cfgH := handler.NewConfigHandler(dataDir)
	projH := handler.NewProjectHandler(projStore, cfgStore, dataDir)

	mux.HandleFunc("GET /api/hub/features", hubH.ListFeatures)
	mux.HandleFunc("GET /api/config", cfgH.Get)
	mux.HandleFunc("PUT /api/config", cfgH.Save)
	mux.HandleFunc("GET /api/models", cfgH.Models)

	// Project routes
	mux.HandleFunc("GET /api/projects", projH.List)
	mux.HandleFunc("GET /api/projects/{id}", projH.Get)
	mux.HandleFunc("POST /api/projects", projH.Create)
	mux.HandleFunc("PUT /api/projects/{id}", projH.Update)
	mux.HandleFunc("DELETE /api/projects/{id}", projH.Delete)
	mux.HandleFunc("POST /api/projects/{id}/repos", projH.AddRepo)
	mux.HandleFunc("DELETE /api/projects/{id}/repos/{repoId}", projH.RemoveRepo)
	mux.HandleFunc("POST /api/projects/{id}/repos/{repoId}/change-branch", projH.ChangeRepoBranch)
	// Deep repo indexing
	mux.HandleFunc("POST /api/projects/{id}/repos/{repoId}/index", projH.IndexRepo)
	mux.HandleFunc("GET /api/projects/{id}/repos/{repoId}/index-status", projH.IndexRepoStatus)
	mux.HandleFunc("DELETE /api/projects/{id}/repos/{repoId}/index", projH.DeleteRepoIndex)
	mux.HandleFunc("GET /api/projects/{id}/index-stream", projH.IndexRepoStream)
	// Legacy single-repo endpoints kept for backward compatibility
	mux.HandleFunc("POST /api/projects/{id}/connect-repo", projH.ConnectRepo)
	mux.HandleFunc("POST /api/projects/{id}/disconnect-repo", projH.DisconnectRepo)
	mux.HandleFunc("POST /api/projects/{id}/change-branch", projH.ChangeBranch)
	mux.HandleFunc("GET /api/projects/{id}/repo-info", projH.RepoInfo)

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
