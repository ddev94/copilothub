package server

import (
	"net/http"
	"copilothub/internal/ui"
)

func frontendHandler() http.Handler {
	return ui.Handler()
}
