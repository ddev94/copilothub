package handler

import (
	"copilothub/internal/config"
	"encoding/json"
	"net/http"
)

type ConfigHandler struct {
	store *config.Store
}

func NewConfigHandler(baseDir string) *ConfigHandler {
	return &ConfigHandler{store: config.NewStore(baseDir)}
}

// Get returns config with sensitive fields masked for security.
func (h *ConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	safe := *cfg
	if safe.AI.Token != "" {
		safe.AI.Token = "***"
	}
	if safe.Knowledge.EmbeddingKey != "" {
		safe.Knowledge.EmbeddingKey = "***"
	}
	writeJSON(w, safe)
}

var providerModels = map[string][]string{
	"copilot":   {"gpt-4o", "gpt-4o-mini", "claude-opus-4.6", "claude-sonnet-4.5", "claude-sonnet-4.6", "o1-mini", "o3-mini", "gpt-4.1"},
	"openai":    {"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "o1", "o1-mini", "o3-mini"},
	"anthropic": {"claude-opus-4-7", "claude-sonnet-4-6", "claude-haiku-4.5"},
}

func (h *ConfigHandler) Models(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	provider := cfg.AI.Provider
	if provider == "" {
		provider = "copilot"
	}
	models := providerModels[provider]
	writeJSON(w, map[string]any{"models": models, "current": cfg.AI.Model})
}

func (h *ConfigHandler) Save(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.store.Save(&cfg); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
