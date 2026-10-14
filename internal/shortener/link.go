// Package shortener contains the core logic: creating short codes, storing links and counting clicks.
// It does not know about HTTP, so it is easy to test.
package shortener

import (
	"context"
	"errors"
	"time"
)

// Link is one shortened URL.
type Link struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	TargetURL string    `json:"targetUrl"`
	Clicks    int64     `json:"clicks"`
	CreatedAt time.Time `json:"createdAt"`
}

// Errors returned by the store and service.
var (
	ErrNotFound    = errors.New("link not found")
	ErrCodeTaken   = errors.New("short code already in use")
	ErrInvalidURL  = errors.New("URL must start with http:// or https:// and have a host")
	ErrInvalidCode = errors.New("custom code must be 3-32 letters, digits, - or _")
)

// Store is where links are saved. MySQLStore is used in production, MemoryStore in tests.
type Store interface {
	Create(ctx context.Context, code, targetURL string) (Link, error)
	GetByCode(ctx context.Context, code string) (Link, error)
	IncrementClicks(ctx context.Context, code string) error
	Recent(ctx context.Context, limit int) ([]Link, error)
}
