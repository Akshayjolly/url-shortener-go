package shortener

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"net/url"
	"regexp"
	"strings"
)

const (
	alphabet   = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/l/I to avoid confusion
	codeLength = 7
	maxRetries = 5
)

var customCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)

// Service holds the business rules on top of a Store.
type Service struct {
	store Store
	// newCode can be swapped in tests to make codes predictable.
	newCode func() (string, error)
}

// NewService creates a Service that generates random codes.
func NewService(store Store) *Service {
	return &Service{store: store, newCode: randomCode}
}

// Shorten validates the URL and saves it. If customCode is empty a random code is generated.
func (s *Service) Shorten(ctx context.Context, rawURL, customCode string) (Link, error) {
	target, err := normaliseURL(rawURL)
	if err != nil {
		return Link{}, err
	}

	if customCode = strings.TrimSpace(customCode); customCode != "" {
		if !customCodePattern.MatchString(customCode) {
			return Link{}, ErrInvalidCode
		}
		return s.store.Create(ctx, customCode, target)
	}

	// Random codes can (very rarely) clash, so retry a few times.
	for i := 0; i < maxRetries; i++ {
		code, err := s.newCode()
		if err != nil {
			return Link{}, err
		}
		link, err := s.store.Create(ctx, code, target)
		if !errors.Is(err, ErrCodeTaken) {
			return link, err
		}
	}
	return Link{}, errors.New("could not generate a unique code, please try again")
}

// Lookup returns a link without counting a click (used for the stats page/API).
func (s *Service) Lookup(ctx context.Context, code string) (Link, error) {
	return s.store.GetByCode(ctx, code)
}

// Resolve returns the link for a redirect and counts the click.
func (s *Service) Resolve(ctx context.Context, code string) (Link, error) {
	link, err := s.store.GetByCode(ctx, code)
	if err != nil {
		return Link{}, err
	}
	if err := s.store.IncrementClicks(ctx, code); err != nil {
		return Link{}, err
	}
	link.Clicks++
	return link, nil
}

// Recent lists the newest links.
func (s *Service) Recent(ctx context.Context, limit int) ([]Link, error) {
	return s.store.Recent(ctx, limit)
}

// normaliseURL accepts only absolute http(s) URLs. "example.com" becomes "https://example.com".
func normaliseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidURL
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(raw) > 2048 {
		return "", ErrInvalidURL
	}
	return u.String(), nil
}

// randomCode builds a code from a cryptographically secure random source.
func randomCode() (string, error) {
	var sb strings.Builder
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < codeLength; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		sb.WriteByte(alphabet[n.Int64()])
	}
	return sb.String(), nil
}
