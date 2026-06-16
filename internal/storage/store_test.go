package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/zjutjh/jxh-go/internal/commands"
	"github.com/zjutjh/jxh-go/internal/scheduler"
	"github.com/zjutjh/jxh-go/internal/storage"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestKnowledgeUpsertMarksChangedVectorPending(t *testing.T) {
	store := newTestStore(t)
	first := storage.KnowledgeEntry{SourceKey: "x", Keyword: "x", EntryType: "knowledge", Answer: "old", Content: "old", Enabled: true, ExactReply: true, AIEnabled: true}
	if err := store.UpsertKnowledgeEntries(context.Background(), []storage.KnowledgeEntry{first}, 1); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	second := storage.KnowledgeEntry{SourceKey: "x", Keyword: "x", EntryType: "knowledge", Answer: "new", Content: "new", Enabled: true, ExactReply: true, AIEnabled: true}
	if err := store.UpsertKnowledgeEntries(context.Background(), []storage.KnowledgeEntry{second}, 2); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got, err := store.ListEnabledKnowledge(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].VectorStatus != storage.VectorStatusPending {
		t.Fatalf("entries = %#v", got)
	}
}

func TestProcessedEventsCleanup(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	old := time.Now().Add(-100 * time.Hour)
	if err := store.MarkProcessedEvent(ctx, "old", old); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkProcessedEvent(ctx, "new", time.Now()); err != nil {
		t.Fatal(err)
	}
	removed, err := store.CleanupProcessedEvents(ctx, time.Now().Add(-72*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d", removed)
	}
}

func TestSeenOrMarkProcessedEvent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seen, err := store.SeenOrMarkProcessedEvent(ctx, "event-1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if seen {
		t.Fatal("first event should not be seen")
	}
	seen, err = store.SeenOrMarkProcessedEvent(ctx, "event-1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("second event should be seen")
	}
}

func TestScheduledJobRuntimeState(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	id, err := store.AddScheduledJob(ctx, commands.ScheduledJobInput{
		Type:     scheduler.JobTypeOnce,
		TimeHHMM: "10:00",
		GroupID:  123,
		Message:  "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := store.ListActiveSchedulerJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].ID != id || jobs[0].Message != "hello" {
		t.Fatalf("jobs = %#v", jobs)
	}
	now := time.Now()
	if err := store.MarkScheduledJobRan(ctx, id, now, true); err != nil {
		t.Fatal(err)
	}
	jobs, err = store.ListActiveSchedulerJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("disabled job should not be active: %#v", jobs)
	}
}

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store := storage.NewStore(db)
	if err := store.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	return store
}
