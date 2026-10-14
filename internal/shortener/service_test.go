package shortener

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestShortenGeneratesCode(t *testing.T) {
	s := NewService(NewMemoryStore())
	link, err := s.Shorten(context.Background(), "https://example.com/page", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(link.Code) != codeLength {
		t.Errorf("code %q should have length %d", link.Code, codeLength)
	}
	if link.TargetURL != "https://example.com/page" {
		t.Errorf("unexpected target %q", link.TargetURL)
	}
}

func TestShortenValidatesURL(t *testing.T) {
	s := NewService(NewMemoryStore())
	// table-driven test: each case is one input and the expected error
	cases := []struct {
		in      string
		wantErr error
		wantURL string
	}{
		{"https://griffith.ie", nil, "https://griffith.ie"},
		{"griffith.ie/courses", nil, "https://griffith.ie/courses"},
		{"http://localhost:8080/x", nil, "http://localhost:8080/x"},
		{"", ErrInvalidURL, ""},
		{"ftp://files.example.com", ErrInvalidURL, ""},
		{"javascript:alert(1)", ErrInvalidURL, ""},
		{"https://", ErrInvalidURL, ""},
	}
	for _, c := range cases {
		link, err := s.Shorten(context.Background(), c.in, "")
		if !errors.Is(err, c.wantErr) {
			t.Errorf("Shorten(%q) error = %v, want %v", c.in, err, c.wantErr)
		}
		if err == nil && link.TargetURL != c.wantURL {
			t.Errorf("Shorten(%q) url = %q, want %q", c.in, link.TargetURL, c.wantURL)
		}
	}
}

func TestCustomCode(t *testing.T) {
	s := NewService(NewMemoryStore())
	ctx := context.Background()
	if _, err := s.Shorten(ctx, "https://a.com", "my-cv"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Shorten(ctx, "https://b.com", "my-cv"); !errors.Is(err, ErrCodeTaken) {
		t.Errorf("want ErrCodeTaken, got %v", err)
	}
	if _, err := s.Shorten(ctx, "https://b.com", "a b"); !errors.Is(err, ErrInvalidCode) {
		t.Errorf("want ErrInvalidCode, got %v", err)
	}
}

func TestRetriesWhenRandomCodeClashes(t *testing.T) {
	store := NewMemoryStore()
	s := NewService(store)
	codes := []string{"same", "same", "other"}
	s.newCode = func() (string, error) {
		c := codes[0]
		codes = codes[1:]
		return c, nil
	}
	ctx := context.Background()
	first, _ := s.Shorten(ctx, "https://a.com", "")
	second, err := s.Shorten(ctx, "https://b.com", "")
	if err != nil || first.Code != "same" || second.Code != "other" {
		t.Fatalf("got %q, %q, %v", first.Code, second.Code, err)
	}
}

func TestResolveCountsClicks(t *testing.T) {
	s := NewService(NewMemoryStore())
	ctx := context.Background()
	link, _ := s.Shorten(ctx, "https://example.com", "abc")
	for i := 0; i < 3; i++ {
		if _, err := s.Resolve(ctx, link.Code); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := s.Lookup(ctx, "abc")
	if got.Clicks != 3 {
		t.Errorf("clicks = %d, want 3", got.Clicks)
	}
	if _, err := s.Resolve(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestConcurrentClicksAreAllCounted(t *testing.T) {
	s := NewService(NewMemoryStore())
	ctx := context.Background()
	s.Shorten(ctx, "https://example.com", "busy")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Resolve(ctx, "busy")
		}()
	}
	wg.Wait()
	if got, _ := s.Lookup(ctx, "busy"); got.Clicks != 100 {
		t.Errorf("clicks = %d, want 100", got.Clicks)
	}
}

func TestRecentNewestFirstWithLimit(t *testing.T) {
	s := NewService(NewMemoryStore())
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		s.Shorten(ctx, fmt.Sprintf("https://site%d.com", i), fmt.Sprintf("code%d", i))
	}
	recent, _ := s.Recent(ctx, 3)
	if len(recent) != 3 || recent[0].Code != "code4" {
		t.Errorf("unexpected recent list: %+v", recent)
	}
}
