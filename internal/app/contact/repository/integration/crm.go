// Package integration contains clients for external systems.
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/model"
)

// CRMClient fetches contacts from the slow, metered external CRM API.
// It implements service.ContactProvider.
type CRMClient struct {
	baseURL string
	http    *http.Client
}

// NewCRMClient creates a client with connection pooling and a timeout.
func NewCRMClient(baseURL string, timeout time.Duration) *CRMClient {
	return &CRMClient{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        256,
				MaxIdleConnsPerHost: 256,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// FetchContact retrieves one contact. It returns model.ErrContactNotFound
// when the CRM has no such contact.
func (c *CRMClient) FetchContact(ctx context.Context, id string) (model.Contact, error) {
	var out model.Contact
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/contacts/"+url.PathEscape(id), nil)
	if err != nil {
		return out, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return out, fmt.Errorf("crm request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return out, model.ErrContactNotFound
	default:
		return out, fmt.Errorf("crm: unexpected status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, fmt.Errorf("crm decode: %w", err)
	}
	return out, nil
}
