// Package handler adapts HTTP requests to use cases. It owns everything
// HTTP-specific: path params, status codes and response bodies.
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/controller/dto"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/controller/usecase"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/model"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/crosscutting/httpx"
)

// Contact handles contact endpoints.
type Contact struct {
	getContact usecase.GetContact
	log        *slog.Logger
}

// NewContact creates the handler.
func NewContact(getContact usecase.GetContact, log *slog.Logger) *Contact {
	return &Contact{getContact: getContact, log: log.With("component", "contact_handler")}
}

// Get handles GET /contacts/{id}.
func (h *Contact) Get(w http.ResponseWriter, r *http.Request) {
	resp, err := h.getContact.Execute(r.Context(), dto.GetContactInput{ID: r.PathValue("id")})
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusOK, resp)
	case errors.Is(err, model.ErrInvalidContactID):
		httpx.WriteJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid contact id"})
	case errors.Is(err, model.ErrContactNotFound):
		httpx.WriteJSON(w, http.StatusNotFound, dto.ErrorResponse{Error: "contact not found"})
	default:
		httpx.WriteJSON(w, http.StatusBadGateway, dto.ErrorResponse{Error: "failed to load contact"})
	}
}
