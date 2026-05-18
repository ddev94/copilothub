package server

import (
	"net/http"
	"spec-designer/internal/ui"
)

func frontendHandler() http.Handler {
	return ui.Handler()
}
