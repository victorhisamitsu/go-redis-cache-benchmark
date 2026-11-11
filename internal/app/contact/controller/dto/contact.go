// Package dto defines the contracts exposed by the HTTP layer.
// Keeping them separate from the domain model lets the API evolve
// without leaking internal fields.
package dto

import (
	"time"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/model"
)

// GetContactInput is the input of the GetContact use case.
type GetContactInput struct {
	ID string
}

// ContactResponse is the JSON returned by GET /contacts/{id}.
type ContactResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Segment   string    `json:"segment"`
	OptIn     bool      `json:"opt_in"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToContactResponse maps the domain model to the response contract.
func ToContactResponse(c model.Contact) ContactResponse {
	return ContactResponse{
		ID:        c.ID,
		Name:      c.Name,
		Phone:     c.Phone,
		Segment:   c.Segment,
		OptIn:     c.OptIn,
		UpdatedAt: c.UpdatedAt,
	}
}

// ErrorResponse is the JSON body returned on errors.
type ErrorResponse struct {
	Error string `json:"error"`
}
