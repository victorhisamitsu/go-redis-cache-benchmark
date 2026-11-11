// Package contacts implements the read path for contact profiles,
// combining a slow external API with a cache-aside layer.
package contacts

import "time"

// Contact is the profile returned by the external CRM API.
type Contact struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Segment   string    `json:"segment"`
	OptIn     bool      `json:"opt_in"`
	UpdatedAt time.Time `json:"updated_at"`
}
