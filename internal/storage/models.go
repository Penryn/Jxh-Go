package storage

import "time"

const (
	VectorStatusPending = "pending"
	VectorStatusReady   = "ready"
	VectorStatusFailed  = "failed"
)

type KnowledgeEntry struct {
	ID                uint64 `gorm:"primaryKey"`
	SourceKey         string `gorm:"size:255;not null;uniqueIndex"`
	Keyword           string `gorm:"size:255;not null"`
	EntryType         string `gorm:"size:32;not null"`
	Path              string `gorm:"size:512"`
	AliasesJSON       string `gorm:"type:json"`
	Category          string `gorm:"size:64"`
	TagsJSON          string `gorm:"type:json"`
	Answer            string `gorm:"type:text;not null"`
	Content           string `gorm:"type:mediumtext;not null"`
	Enabled           bool   `gorm:"not null"`
	ExactReply        bool   `gorm:"not null"`
	AIEnabled         bool   `gorm:"not null"`
	ContentHash       string `gorm:"size:64;not null"`
	VectorStatus      string `gorm:"size:16;not null;default:pending"`
	VectorContentHash string `gorm:"size:64"`
	VectorSyncedAt    *time.Time
	LastImportRunID   uint64
	SourceUpdatedAt   *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type KnowledgeImportRun struct {
	ID           uint64 `gorm:"primaryKey"`
	Source       string `gorm:"size:32;not null"`
	Status       string `gorm:"size:16;not null"`
	TotalRows    int
	ImportedRows int
	SkippedRows  int
	ErrorMessage string `gorm:"type:text"`
	StartedAt    time.Time
	FinishedAt   *time.Time
}

type Admin struct {
	UserID    int64 `gorm:"primaryKey"`
	CreatedAt time.Time
}

type Blacklist struct {
	UserID    int64 `gorm:"primaryKey"`
	CreatedAt time.Time
}

type ScheduledJob struct {
	ID        uint64 `gorm:"primaryKey"`
	Type      string `gorm:"size:16;not null"`
	TimeHHMM  string `gorm:"size:5;not null"`
	GroupID   int64  `gorm:"not null"`
	Message   string `gorm:"type:text;not null"`
	Enabled   bool   `gorm:"not null"`
	LastRunAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProcessedEvent struct {
	EventKey    string    `gorm:"size:128;primaryKey"`
	ProcessedAt time.Time `gorm:"index"`
}
