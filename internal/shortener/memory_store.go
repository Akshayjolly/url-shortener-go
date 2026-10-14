package shortener

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryStore keeps links in a map. It is safe for concurrent use and is used by the tests
// (and can be used to run the app without MySQL: STORE=memory).
type MemoryStore struct {
	mu     sync.Mutex
	links  map[string]*Link
	nextID int64
}

// NewMemoryStore creates an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{links: make(map[string]*Link)}
}

func (m *MemoryStore) Create(_ context.Context, code, targetURL string) (Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.links[code]; exists {
		return Link{}, ErrCodeTaken
	}
	m.nextID++
	l := &Link{ID: m.nextID, Code: code, TargetURL: targetURL, CreatedAt: time.Now()}
	m.links[code] = l
	return *l, nil
}

func (m *MemoryStore) GetByCode(_ context.Context, code string) (Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.links[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	return *l, nil
}

func (m *MemoryStore) IncrementClicks(_ context.Context, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.links[code]
	if !ok {
		return ErrNotFound
	}
	l.Clicks++
	return nil
}

func (m *MemoryStore) Recent(_ context.Context, limit int) ([]Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Link, 0, len(m.links))
	for _, l := range m.links {
		out = append(out, *l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID }) // newest first
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
