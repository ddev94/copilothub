package handler

import (
	"aikit/internal/hub"
	"encoding/json"
	"net/http"
)

type HubHandler struct {
	registry *hub.Registry
}

func NewHubHandler(registry *hub.Registry) *HubHandler {
	return &HubHandler{registry: registry}
}

func (h *HubHandler) ListFeatures(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"features": h.registry.Manifests(),
	})
}
