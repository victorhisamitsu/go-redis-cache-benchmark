package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/controller/dto"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/model"
)

type fakeService struct {
	calls int
	err   error
}

func (f *fakeService) GetByID(_ context.Context, id string) (model.Contact, error) {
	f.calls++
	return model.Contact{ID: id, Name: "Contact " + id}, f.err
}

func TestExecute_MapsContactToResponse(t *testing.T) {
	uc := NewGetContact(&fakeService{})
	got, err := uc.Execute(context.Background(), dto.GetContactInput{ID: "42"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "42" || got.Name != "Contact 42" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestExecute_RejectsInvalidIDWithoutCallingService(t *testing.T) {
	for _, id := range []string{"", "../etc/passwd", "a b", string(make([]byte, 65))} {
		svc := &fakeService{}
		_, err := NewGetContact(svc).Execute(context.Background(), dto.GetContactInput{ID: id})
		if !errors.Is(err, model.ErrInvalidContactID) {
			t.Errorf("id %q: err = %v, want ErrInvalidContactID", id, err)
		}
		if svc.calls != 0 {
			t.Errorf("id %q: service must not be called for invalid input", id)
		}
	}
}

func TestExecute_PropagatesServiceErrors(t *testing.T) {
	uc := NewGetContact(&fakeService{err: model.ErrContactNotFound})
	if _, err := uc.Execute(context.Background(), dto.GetContactInput{ID: "1"}); !errors.Is(err, model.ErrContactNotFound) {
		t.Fatalf("err = %v, want ErrContactNotFound", err)
	}
}
