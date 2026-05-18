package server

import (
	"net/http"
	"aikit/internal/ui"
)

func frontendHandler() http.Handler {
	return ui.Handler()
}
