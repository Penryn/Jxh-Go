package scheduler

import (
	"context"
	"sync"
	"time"
)

const (
	JobTypeDaily = "每天"
	JobTypeOnce  = "单次"
)

type SendFunc func(context.Context, int64, string) error

type Options struct {
	Send SendFunc
}

type Job struct {
	ID          uint64
	Type        string
	GroupID     int64
	Message     string
	TimeHHMM    string
	RunAt       time.Time
	Enabled     bool
	LastRunAt   *time.Time
	lastRunDate string
}

type Runner struct {
	mu   sync.Mutex
	send SendFunc
	jobs []Job
}

func New(opts Options) *Runner {
	return &Runner{send: opts.Send}
}

func (r *Runner) AddForTest(job Job) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, job)
}

func (r *Runner) RunDue(ctx context.Context, now time.Time) {
	r.mu.Lock()
	jobs := make([]Job, len(r.jobs))
	copy(jobs, r.jobs)
	r.mu.Unlock()
	for i := range jobs {
		job := jobs[i]
		if !job.Enabled || !isDue(job, now) {
			continue
		}
		if r.send != nil {
			_ = r.send(ctx, job.GroupID, job.Message)
		}
		r.markRan(job.ID, now)
	}
}

func (r *Runner) markRan(id uint64, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.jobs {
		if r.jobs[i].ID != id {
			continue
		}
		if r.jobs[i].Type == JobTypeOnce {
			r.jobs[i].Enabled = false
		}
		r.jobs[i].LastRunAt = &now
		r.jobs[i].lastRunDate = now.Format("2006-01-02")
	}
}

func isDue(job Job, now time.Time) bool {
	return IsDue(job, now)
}

func IsDue(job Job, now time.Time) bool {
	if !job.Enabled {
		return false
	}
	switch job.Type {
	case JobTypeOnce:
		if alreadyRanToday(job, now) {
			return false
		}
		if !job.RunAt.IsZero() {
			return !job.RunAt.After(now)
		}
		return job.TimeHHMM != "" && now.Format("15:04") >= job.TimeHHMM
	case JobTypeDaily:
		if job.TimeHHMM == "" || alreadyRanToday(job, now) {
			return false
		}
		return now.Format("15:04") >= job.TimeHHMM
	default:
		return false
	}
}

func alreadyRanToday(job Job, now time.Time) bool {
	today := now.Format("2006-01-02")
	if job.lastRunDate == today {
		return true
	}
	return job.LastRunAt != nil && job.LastRunAt.In(now.Location()).Format("2006-01-02") == today
}
