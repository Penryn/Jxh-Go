package cache_test

import (
	"testing"
	"time"

	"github.com/zjutjh/jxh-go/internal/cache"
	"github.com/zjutjh/jxh-go/internal/knowledge"
)

func TestKnowledgeCacheAtomicReplace(t *testing.T) {
	c := cache.NewKnowledge()
	c.Replace(knowledge.NewKeywordIndex([]knowledge.Entry{{Keyword: "旧", Answer: "old", Enabled: true, ExactReply: true}}))
	c.Replace(knowledge.NewKeywordIndex([]knowledge.Entry{{Keyword: "新", Answer: "new", Enabled: true, ExactReply: true}}))
	if _, ok := c.Lookup("旧"); ok {
		t.Fatal("old index still visible")
	}
	got, ok := c.Lookup("新")
	if !ok || got.Answer != "new" {
		t.Fatalf("new lookup failed: %#v %v", got, ok)
	}
}

func TestEventDedupeExpires(t *testing.T) {
	d := cache.NewEventDedupe(20 * time.Millisecond)
	if d.SeenOrMark("event-1") {
		t.Fatal("first mark should be false")
	}
	if !d.SeenOrMark("event-1") {
		t.Fatal("second mark should be true")
	}
	time.Sleep(30 * time.Millisecond)
	if removed := d.Cleanup(time.Now()); removed != 1 {
		t.Fatalf("removed = %d", removed)
	}
	if d.SeenOrMark("event-1") {
		t.Fatal("expired event should not be seen")
	}
}
