package cache

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/zjutjh/jxh-go/internal/knowledge"
)

type Knowledge struct {
	index atomic.Pointer[knowledge.KeywordIndex]
}

func NewKnowledge() *Knowledge {
	k := &Knowledge{}
	k.Replace(knowledge.NewKeywordIndex(nil))
	return k
}

func (k *Knowledge) Replace(index *knowledge.KeywordIndex) {
	if index == nil {
		index = knowledge.NewKeywordIndex(nil)
	}
	k.index.Store(index)
}

func (k *Knowledge) Lookup(message string) (knowledge.Entry, bool) {
	idx := k.index.Load()
	if idx == nil {
		return knowledge.Entry{}, false
	}
	return idx.Lookup(message)
}

type EventDedupe struct {
	mu        sync.Mutex
	retention time.Duration
	seen      map[string]time.Time
}

func NewEventDedupe(retention time.Duration) *EventDedupe {
	if retention <= 0 {
		retention = 72 * time.Hour
	}
	return &EventDedupe{retention: retention, seen: make(map[string]time.Time)}
}

func (d *EventDedupe) SeenOrMark(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	if at, ok := d.seen[key]; ok && now.Sub(at) <= d.retention {
		return true
	}
	d.seen[key] = now
	return false
}

func (d *EventDedupe) Cleanup(now time.Time) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	var removed int
	for key, at := range d.seen {
		if now.Sub(at) > d.retention {
			delete(d.seen, key)
			removed++
		}
	}
	return removed
}
