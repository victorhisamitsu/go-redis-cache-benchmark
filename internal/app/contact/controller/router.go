// Package controller exposes the contact module over HTTP.
package controller

import (
	"net/http"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/controller/handler"
)

// RegisterRoutes mounts the contact routes on mux.
func RegisterRoutes(mux *http.ServeMux, h *handler.Contact) {
	mux.HandleFunc("GET /contacts/{id}", h.Get)
}
