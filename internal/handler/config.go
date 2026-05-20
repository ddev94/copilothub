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
