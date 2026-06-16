package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/zjutjh/jxh-go/internal/commands"
	"github.com/zjutjh/jxh-go/internal/knowledge"
	"github.com/zjutjh/jxh-go/internal/scheduler"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(
		&KnowledgeEntry{},
		&KnowledgeImportRun{},
		&Admin{},
		&Blacklist{},
		&ScheduledJob{},
		&ProcessedEvent{},
	)
}

func (s *Store) UpsertKnowledgeEntries(ctx context.Context, entries []KnowledgeEntry, runID uint64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for _, entry := range entries {
			entry.LastImportRunID = runID
			entry.ContentHash = hashContent(entry.Content)
			if entry.VectorStatus == "" {
				entry.VectorStatus = VectorStatusPending
			}
			var existing KnowledgeEntry
			err := tx.Where("source_key = ?", entry.SourceKey).Take(&existing).Error
			if err == nil {
				if existing.ContentHash == entry.ContentHash && existing.VectorStatus != "" {
					entry.VectorStatus = existing.VectorStatus
					entry.VectorContentHash = existing.VectorContentHash
					entry.VectorSyncedAt = existing.VectorSyncedAt
				} else {
					entry.VectorStatus = VectorStatusPending
					entry.VectorContentHash = ""
					entry.VectorSyncedAt = nil
				}
				entry.ID = existing.ID
				entry.CreatedAt = existing.CreatedAt
				if err := tx.Save(&entry).Error; err != nil {
					return err
				}
				continue
			}
			if err != gorm.ErrRecordNotFound {
				return err
			}
			entry.CreatedAt = now
			entry.UpdatedAt = now
			if err := tx.Create(&entry).Error; err != nil {
				return err
			}
		}
		if runID != 0 {
			return tx.Model(&KnowledgeEntry{}).
				Where("last_import_run_id <> ? OR last_import_run_id IS NULL", runID).
				Update("enabled", false).Error
		}
		return nil
	})
}

func (s *Store) ListEnabledKnowledge(ctx context.Context) ([]KnowledgeEntry, error) {
	var entries []KnowledgeEntry
	err := s.db.WithContext(ctx).Where("enabled = ?", true).Order("id ASC").Find(&entries).Error
	return entries, err
}

func (s *Store) AddAdmin(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&Admin{UserID: userID}).Error
}

func (s *Store) RemoveAdmin(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).Delete(&Admin{}, "user_id = ?", userID).Error
}

func (s *Store) ClearAdmins(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&Admin{}).Error
}

func (s *Store) ListAdmins(ctx context.Context) ([]int64, error) {
	var users []int64
	err := s.db.WithContext(ctx).Model(&Admin{}).Order("user_id ASC").Pluck("user_id", &users).Error
	return users, err
}

func (s *Store) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&Admin{}).Where("user_id = ?", userID).Count(&count).Error
	return count > 0, err
}

func (s *Store) AddBlacklist(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&Blacklist{UserID: userID}).Error
}

func (s *Store) RemoveBlacklist(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).Delete(&Blacklist{}, "user_id = ?", userID).Error
}

func (s *Store) ClearBlacklist(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&Blacklist{}).Error
}

func (s *Store) ListBlacklist(ctx context.Context) ([]int64, error) {
	var users []int64
	err := s.db.WithContext(ctx).Model(&Blacklist{}).Order("user_id ASC").Pluck("user_id", &users).Error
	return users, err
}

func (s *Store) IsBlacklisted(ctx context.Context, userID int64) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&Blacklist{}).Where("user_id = ?", userID).Count(&count).Error
	return count > 0, err
}

func (s *Store) MarkProcessedEvent(ctx context.Context, key string, at time.Time) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"processed_at"}),
	}).Create(&ProcessedEvent{EventKey: key, ProcessedAt: at}).Error
}

func (s *Store) HasProcessedEvent(ctx context.Context, key string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&ProcessedEvent{}).Where("event_key = ?", key).Count(&count).Error
	return count > 0, err
}

func (s *Store) CleanupProcessedEvents(ctx context.Context, before time.Time) (int64, error) {
	result := s.db.WithContext(ctx).Where("processed_at < ?", before).Delete(&ProcessedEvent{})
	return result.RowsAffected, result.Error
}

func (s *Store) SeenOrMarkProcessedEvent(ctx context.Context, key string, at time.Time) (bool, error) {
	var existing ProcessedEvent
	err := s.db.WithContext(ctx).Where("event_key = ?", key).Take(&existing).Error
	if err == nil {
		return true, nil
	}
	if err != gorm.ErrRecordNotFound {
		return false, err
	}
	return false, s.MarkProcessedEvent(ctx, key, at)
}

func (s *Store) ListScheduledJobs(ctx context.Context) ([]commands.ScheduledJobView, error) {
	var jobs []ScheduledJob
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Order("id ASC").Find(&jobs).Error; err != nil {
		return nil, err
	}
	out := make([]commands.ScheduledJobView, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, commands.ScheduledJobView{ID: job.ID, Type: job.Type, TimeHHMM: job.TimeHHMM, GroupID: job.GroupID, Message: job.Message, Enabled: job.Enabled})
	}
	return out, nil
}

func (s *Store) AddScheduledJob(ctx context.Context, input commands.ScheduledJobInput) (uint64, error) {
	job := ScheduledJob{Type: input.Type, TimeHHMM: input.TimeHHMM, GroupID: input.GroupID, Message: input.Message, Enabled: true}
	err := s.db.WithContext(ctx).Create(&job).Error
	return job.ID, err
}

func (s *Store) RemoveScheduledJob(ctx context.Context, id uint64) error {
	return s.db.WithContext(ctx).Model(&ScheduledJob{}).Where("id = ?", id).Update("enabled", false).Error
}

func (s *Store) ListActiveSchedulerJobs(ctx context.Context) ([]scheduler.Job, error) {
	var jobs []ScheduledJob
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Order("id ASC").Find(&jobs).Error; err != nil {
		return nil, err
	}
	out := make([]scheduler.Job, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, scheduler.Job{
			ID:        job.ID,
			Type:      job.Type,
			GroupID:   job.GroupID,
			Message:   job.Message,
			TimeHHMM:  job.TimeHHMM,
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
	return s.db.WithContext(ctx).Model(&ScheduledJob{}).Where("id = ?", id).Updates(updates).Error
}

func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
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
