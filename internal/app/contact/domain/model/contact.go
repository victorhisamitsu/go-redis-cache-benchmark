// Package model holds the contact domain entities and errors.
package model

import (
	"errors"
	"time"
)

// Domain errors. Outer layers translate them into transport-specific
// responses (e.g. HTTP status codes) — the domain never knows about HTTP.
var (
	ErrContactNotFound  = errors.New("contact not found")
	ErrInvalidContactID = errors.New("invalid contact id")
)

// Contact is the profile owned by the external CRM.
type Contact struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Segment   string    `json:"segment"`
	OptIn     bool      `json:"opt_in"`
	UpdatedAt time.Time `json:"updated_at"`
}
