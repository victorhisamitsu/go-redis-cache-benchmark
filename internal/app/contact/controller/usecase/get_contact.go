// Package usecase orchestrates application workflows: it validates input,
// calls domain services and maps results to DTOs.
package usecase

import (
	"context"
	"fmt"
	"regexp"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/controller/dto"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/model"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/service"
)

var validID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// GetContact returns a contact profile.
type GetContact interface {
	Execute(ctx context.Context, in dto.GetContactInput) (dto.ContactResponse, error)
}

type getContact struct {
	contacts service.ContactService
}

// NewGetContact creates the use case.
func NewGetContact(s service.ContactService) GetContact {
	return &getContact{contacts: s}
}

// Execute validates the id, fetches the contact and maps it to the response.
func (uc *getContact) Execute(ctx context.Context, in dto.GetContactInput) (dto.ContactResponse, error) {
	if !validID.MatchString(in.ID) {
		return dto.ContactResponse{}, fmt.Errorf("%w: %q", model.ErrInvalidContactID, in.ID)
	}
	c, err := uc.contacts.GetByID(ctx, in.ID)
	if err != nil {
		return dto.ContactResponse{}, err
	}
	return dto.ToContactResponse(c), nil
}
