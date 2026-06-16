package vector

import (
	"context"
	"math"
	"sort"
	"sync"
)

type Meta struct {
	Category string
}

type SearchResult struct {
	EntryID int64
	Score   float64
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

type Store interface {
	Upsert(ctx context.Context, entryID int64, embedding []float64, meta Meta) error
	Search(ctx context.Context, embedding []float64, topK int, threshold float64) ([]SearchResult, error)
	Delete(ctx context.Context, entryIDs []int64) error
	Close() error
}

type NoopStore struct{}

func (NoopStore) Upsert(context.Context, int64, []float64, Meta) error { return nil }
func (NoopStore) Search(context.Context, []float64, int, float64) ([]SearchResult, error) {
	return nil, nil
}
func (NoopStore) Delete(context.Context, []int64) error { return nil }
func (NoopStore) Close() error                          { return nil }

type MemoryStore struct {
	mu      sync.Mutex
	vectors map[int64][]float64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{vectors: map[int64][]float64{}}
}

func (s *MemoryStore) Upsert(ctx context.Context, entryID int64, embedding []float64, meta Meta) error {
	_ = ctx
	_ = meta
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := append([]float64(nil), embedding...)
	s.vectors[entryID] = cp
	return nil
}

func (s *MemoryStore) Search(ctx context.Context, embedding []float64, topK int, threshold float64) ([]SearchResult, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if topK <= 0 {
		topK = 5
	}
	results := make([]SearchResult, 0, len(s.vectors))
	for id, candidate := range s.vectors {
		score := cosine(embedding, candidate)
		if score >= threshold {
			results = append(results, SearchResult{EntryID: id, Score: score})
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func (s *MemoryStore) Delete(ctx context.Context, entryIDs []int64) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range entryIDs {
		delete(s.vectors, id)
	}
	return nil
}

func (s *MemoryStore) Close() error { return nil }

func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, aa, bb float64
	for i := range a {
		dot += a[i] * b[i]
		aa += a[i] * a[i]
		bb += b[i] * b[i]
	}
	if aa == 0 || bb == 0 {
		return 0
	}
	return dot / (math.Sqrt(aa) * math.Sqrt(bb))
}
