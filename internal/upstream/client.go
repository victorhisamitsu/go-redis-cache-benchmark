// Package upstream is the HTTP client for the slow external CRM API.
package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/contacts"
)

// ErrNotFound is returned when the external API has no such contact.
var ErrNotFound = errors.New("upstream: contact not found")

// Client calls the external API.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient creates a client with a sane timeout.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        256,
				MaxIdleConnsPerHost: 256,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// FetchContact retrieves one contact from the external API.
func (c *Client) FetchContact(ctx context.Context, id string) (contacts.Contact, error) {
	var out contacts.Contact
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/contacts/"+url.PathEscape(id), nil)
	if err != nil {
		return out, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return out, fmt.Errorf("upstream request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return out, ErrNotFound
	default:
		return out, fmt.Errorf("upstream: unexpected status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, fmt.Errorf("upstream decode: %w", err)
	}
	return out, nil
}
