package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/controller/dto"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/model"
)

type fakeUseCase struct{ err error }

func (f fakeUseCase) Execute(_ context.Context, in dto.GetContactInput) (dto.ContactResponse, error) {
	return dto.ContactResponse{ID: in.ID}, f.err
}

func TestGet_StatusCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"ok", nil, http.StatusOK},
		{"invalid id", model.ErrInvalidContactID, http.StatusBadRequest},
		{"not found", model.ErrContactNotFound, http.StatusNotFound},
		{"upstream failure", errors.New("timeout"), http.StatusBadGateway},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /contacts/{id}", NewContact(fakeUseCase{err: tc.err}, log).Get)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/contacts/42", nil))

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("content-type = %q", ct)
			}
		})
	}
}
