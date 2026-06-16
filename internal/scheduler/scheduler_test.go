package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/zjutjh/jxh-go/internal/scheduler"
)

func TestSchedulerRunsSingleJobOnce(t *testing.T) {
	var sent int
	runner := scheduler.New(scheduler.Options{Send: func(context.Context, int64, string) error {
		sent++
		return nil
	}})
	runner.AddForTest(scheduler.Job{ID: 1, Type: scheduler.JobTypeOnce, GroupID: 1, Message: "hello", RunAt: time.Now().Add(-time.Second), Enabled: true})
	runner.RunDue(context.Background(), time.Now())
	runner.RunDue(context.Background(), time.Now())
	if sent != 1 {
		t.Fatalf("sent = %d", sent)
	}
}

func TestDailyJobSkipsWhenLastRunToday(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.Local)
	lastRun := time.Date(2026, 6, 16, 9, 0, 0, 0, time.Local)
	job := scheduler.Job{ID: 1, Type: scheduler.JobTypeDaily, TimeHHMM: "10:00", Enabled: true, LastRunAt: &lastRun}
	if scheduler.IsDue(job, now) {
		t.Fatal("job should not be due twice in one day")
	}
}

func TestSingleJobWithHHMMRunsOnceToday(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.Local)
	job := scheduler.Job{ID: 1, Type: scheduler.JobTypeOnce, TimeHHMM: "10:00", Enabled: true}
	if !scheduler.IsDue(job, now) {
		t.Fatal("single job should be due after HH:MM")
	}
	lastRun := time.Date(2026, 6, 16, 12, 0, 0, 0, time.Local)
	job.LastRunAt = &lastRun
	if scheduler.IsDue(job, now) {
		t.Fatal("single job should not rerun after last_run_at is set")
	}
}
