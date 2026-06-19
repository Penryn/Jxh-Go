package storage

import (
	"context"
	"time"

	"github.com/zjutjh/jxh-go/internal/commands"
	"github.com/zjutjh/jxh-go/internal/knowledge"
	"github.com/zjutjh/jxh-go/internal/scheduler"
	storagemodel "github.com/zjutjh/jxh-go/internal/storage/model"
	"github.com/zjutjh/jxh-go/internal/storage/query"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db *gorm.DB
	q  *query.Query
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db, q: query.Use(db)}
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

func (s *Store) UpsertKnowledgeEntries(ctx context.Context, entries []KnowledgeEntry, runID uint64) error {
	return s.q.Transaction(func(tx *query.Query) error {
		now := time.Now()
		ke := tx.KnowledgeEntry
		for _, entry := range entries {
			entry.LastImportRunID = runID
			existingModel, err := ke.WithContext(ctx).Where(ke.SourceKey.Eq(entry.SourceKey)).Take()
			if err == nil {
				existing := knowledgeEntryFromModel(existingModel)
				entry.ID = existing.ID
				entry.CreatedAt = existing.CreatedAt
				if err := ke.WithContext(ctx).Save(knowledgeEntryToModel(entry)); err != nil {
					return err
				}
				continue
			}
			if err != gorm.ErrRecordNotFound {
				return err
			}
			entry.CreatedAt = now
			entry.UpdatedAt = now
			if err := ke.WithContext(ctx).Create(knowledgeEntryToModel(entry)); err != nil {
				return err
			}
		}
		if runID != 0 {
			_, err := ke.WithContext(ctx).
				Where(ke.LastImportRunID.Neq(runID)).
				Or(ke.LastImportRunID.IsNull()).
				Update(ke.Enabled, false)
			return err
		}
		return nil
	})
}

func (s *Store) UpsertKnowledge(ctx context.Context, entries []knowledge.Entry, runID uint64) error {
	return s.UpsertKnowledgeEntries(ctx, FromKnowledgeEntries(entries), runID)
}

func (s *Store) ListEnabledKnowledge(ctx context.Context) ([]KnowledgeEntry, error) {
	ke := s.q.KnowledgeEntry
	entries, err := ke.WithContext(ctx).Where(ke.Enabled.Is(true)).Order(ke.ID).Find()
	if err != nil {
		return nil, err
	}
	return knowledgeEntriesFromModels(entries), nil
}

func (s *Store) AddAdmin(ctx context.Context, userID int64) error {
	return s.q.Admin.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&storagemodel.Admin{UserID: userID})
}

func (s *Store) RemoveAdmin(ctx context.Context, userID int64) error {
	admin := s.q.Admin
	_, err := admin.WithContext(ctx).Where(admin.UserID.Eq(userID)).Delete()
	return err
}

func (s *Store) ClearAdmins(ctx context.Context) error {
	_, err := s.q.Admin.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Delete()
	return err
}

func (s *Store) ListAdmins(ctx context.Context) ([]int64, error) {
	admin := s.q.Admin
	admins, err := admin.WithContext(ctx).Order(admin.UserID).Find()
	if err != nil {
		return nil, err
	}
	users := make([]int64, 0, len(admins))
	for _, admin := range admins {
		users = append(users, admin.UserID)
	}
	return users, nil
}

func (s *Store) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	admin := s.q.Admin
	count, err := admin.WithContext(ctx).Where(admin.UserID.Eq(userID)).Count()
	return count > 0, err
}

func (s *Store) AddBlacklist(ctx context.Context, userID int64) error {
	return s.q.Blacklist.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&storagemodel.Blacklist{UserID: userID})
}

func (s *Store) RemoveBlacklist(ctx context.Context, userID int64) error {
	blacklist := s.q.Blacklist
	_, err := blacklist.WithContext(ctx).Where(blacklist.UserID.Eq(userID)).Delete()
	return err
}

func (s *Store) ClearBlacklist(ctx context.Context) error {
	_, err := s.q.Blacklist.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Delete()
	return err
}

func (s *Store) ListBlacklist(ctx context.Context) ([]int64, error) {
	blacklist := s.q.Blacklist
	blacklists, err := blacklist.WithContext(ctx).Order(blacklist.UserID).Find()
	if err != nil {
		return nil, err
	}
	users := make([]int64, 0, len(blacklists))
	for _, blacklist := range blacklists {
		users = append(users, blacklist.UserID)
	}
	return users, nil
}

func (s *Store) IsBlacklisted(ctx context.Context, userID int64) (bool, error) {
	blacklist := s.q.Blacklist
	count, err := blacklist.WithContext(ctx).Where(blacklist.UserID.Eq(userID)).Count()
	return count > 0, err
}

func (s *Store) MarkProcessedEvent(ctx context.Context, key string, at time.Time) error {
	return s.q.ProcessedEvent.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"processed_at"}),
	}).Create(&storagemodel.ProcessedEvent{EventKey: key, ProcessedAt: &at})
}

func (s *Store) HasProcessedEvent(ctx context.Context, key string) (bool, error) {
	event := s.q.ProcessedEvent
	count, err := event.WithContext(ctx).Where(event.EventKey.Eq(key)).Count()
	return count > 0, err
}

func (s *Store) CleanupProcessedEvents(ctx context.Context, before time.Time) (int64, error) {
	event := s.q.ProcessedEvent
	result, err := event.WithContext(ctx).Where(event.ProcessedAt.Lt(before)).Delete()
	return result.RowsAffected, err
}

func (s *Store) SeenOrMarkProcessedEvent(ctx context.Context, key string, at time.Time) (bool, error) {
	event := s.q.ProcessedEvent
	_, err := event.WithContext(ctx).Where(event.EventKey.Eq(key)).Take()
	if err == nil {
		return true, nil
	}
	if err != gorm.ErrRecordNotFound {
		return false, err
	}
	return false, s.MarkProcessedEvent(ctx, key, at)
}

func (s *Store) ListScheduledJobs(ctx context.Context) ([]commands.ScheduledJobView, error) {
	job := s.q.ScheduledJob
	jobs, err := job.WithContext(ctx).Where(job.Enabled.Is(true)).Order(job.ID).Find()
	if err != nil {
		return nil, err
	}
	out := make([]commands.ScheduledJobView, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, commands.ScheduledJobView{ID: job.ID, Type: job.Type, TimeHHMM: job.TimeHhmm, GroupID: job.GroupID, Message: job.Message, Enabled: job.Enabled})
	}
	return out, nil
}

func (s *Store) AddScheduledJob(ctx context.Context, input commands.ScheduledJobInput) (uint64, error) {
	job := &storagemodel.ScheduledJob{Type: input.Type, TimeHhmm: input.TimeHHMM, GroupID: input.GroupID, Message: input.Message, Enabled: true}
	err := s.q.ScheduledJob.WithContext(ctx).Create(job)
	return job.ID, err
}

func (s *Store) RemoveScheduledJob(ctx context.Context, id uint64) error {
	job := s.q.ScheduledJob
	_, err := job.WithContext(ctx).Where(job.ID.Eq(id)).Update(job.Enabled, false)
	return err
}

func (s *Store) ListActiveSchedulerJobs(ctx context.Context) ([]scheduler.Job, error) {
	job := s.q.ScheduledJob
	jobs, err := job.WithContext(ctx).Where(job.Enabled.Is(true)).Order(job.ID).Find()
	if err != nil {
		return nil, err
	}
	out := make([]scheduler.Job, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, scheduler.Job{
			ID:        job.ID,
			Type:      job.Type,
			GroupID:   job.GroupID,
			Message:   job.Message,
			TimeHHMM:  job.TimeHhmm,
			Enabled:   job.Enabled,
			LastRunAt: job.LastRunAt,
		})
	}
	return out, nil
}

func (s *Store) MarkScheduledJobRan(ctx context.Context, id uint64, at time.Time, disable bool) error {
	updates := map[string]any{"last_run_at": at}
	if disable {
		updates["enabled"] = false
	}
	job := s.q.ScheduledJob
	_, err := job.WithContext(ctx).Where(job.ID.Eq(id)).Updates(updates)
	return err
}

func knowledgeEntriesFromModels(entries []*storagemodel.KnowledgeEntry) []KnowledgeEntry {
	out := make([]KnowledgeEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, knowledgeEntryFromModel(entry))
	}
	return out
}

func knowledgeEntryToModel(entry KnowledgeEntry) *storagemodel.KnowledgeEntry {
	return &storagemodel.KnowledgeEntry{
		ID:              entry.ID,
		SourceKey:       entry.SourceKey,
		Keyword:         entry.Keyword,
		EntryType:       entry.EntryType,
		Path:            stringPtr(entry.Path),
		AliasesJSON:     stringPtr(entry.AliasesJSON),
		Category:        stringPtr(entry.Category),
		TagsJSON:        stringPtr(entry.TagsJSON),
		Answer:          entry.Answer,
		Content:         entry.Content,
		Enabled:         entry.Enabled,
		ExactReply:      entry.ExactReply,
		AiEnabled:       entry.AIEnabled,
		LastImportRunID: uint64Ptr(entry.LastImportRunID),
		SourceUpdatedAt: entry.SourceUpdatedAt,
		CreatedAt:       timePtr(entry.CreatedAt),
		UpdatedAt:       timePtr(entry.UpdatedAt),
	}
}

func knowledgeEntryFromModel(entry *storagemodel.KnowledgeEntry) KnowledgeEntry {
	if entry == nil {
		return KnowledgeEntry{}
	}
	return KnowledgeEntry{
		ID:              entry.ID,
		SourceKey:       entry.SourceKey,
		Keyword:         entry.Keyword,
		EntryType:       entry.EntryType,
		Path:            stringFromPtr(entry.Path),
		AliasesJSON:     stringFromPtr(entry.AliasesJSON),
		Category:        stringFromPtr(entry.Category),
		TagsJSON:        stringFromPtr(entry.TagsJSON),
		Answer:          entry.Answer,
		Content:         entry.Content,
		Enabled:         entry.Enabled,
		ExactReply:      entry.ExactReply,
		AIEnabled:       entry.AiEnabled,
		LastImportRunID: uint64FromPtr(entry.LastImportRunID),
		SourceUpdatedAt: entry.SourceUpdatedAt,
		CreatedAt:       timeFromPtr(entry.CreatedAt),
		UpdatedAt:       timeFromPtr(entry.UpdatedAt),
	}
}

func stringFromPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPtr(value string) *string {
	return &value
}

func uint64FromPtr(value *uint64) uint64 {
	if value == nil {
		return 0
	}
	return *value
}

func uint64Ptr(value uint64) *uint64 {
	return &value
}

func timeFromPtr(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func FromKnowledgeEntries(entries []knowledge.Entry) []KnowledgeEntry {
	out := make([]KnowledgeEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, KnowledgeEntry{
			SourceKey:   entry.SourceKey,
			Keyword:     entry.Keyword,
			EntryType:   entry.EntryType,
			Path:        entry.Path,
			AliasesJSON: jsonList(entry.Aliases),
			Category:    entry.Category,
			TagsJSON:    jsonList(entry.Tags),
			Answer:      entry.Answer,
			Content:     entry.Content,
			Enabled:     entry.Enabled,
			ExactReply:  entry.ExactReply,
			AIEnabled:   entry.AIEnabled,
		})
	}
	return out
}

func ToKnowledgeEntries(entries []KnowledgeEntry) []knowledge.Entry {
	out := make([]knowledge.Entry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, knowledge.Entry{
			SourceKey:  entry.SourceKey,
			Keyword:    entry.Keyword,
			EntryType:  entry.EntryType,
			Path:       entry.Path,
			Aliases:    parseJSONList(entry.AliasesJSON),
			Category:   entry.Category,
			Tags:       parseJSONList(entry.TagsJSON),
			Answer:     entry.Answer,
			Content:    entry.Content,
			Enabled:    entry.Enabled,
			ExactReply: entry.ExactReply,
			AIEnabled:  entry.AIEnabled,
		})
	}
	return out
}
