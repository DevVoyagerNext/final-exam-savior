package model

import "time"

type PreviewConversionTask struct {
	ID                uint64  `gorm:"primaryKey;autoIncrement"`
	FileID            *uint64 `gorm:"index:idx_preview_conversion_file,priority:1"`
	RequestUserID     uint64  `gorm:"not null"`
	SourceFileType    string  `gorm:"size:128;not null"`
	Status            string  `gorm:"size:32;not null;index:idx_preview_conversion_status,priority:1"`
	AutoRetryCount    uint32  `gorm:"not null;default:0"`
	ManualRetryCount  uint32  `gorm:"not null;default:0"`
	MaxAutoRetryCount uint32  `gorm:"not null;default:3"`
	RetryIntervalSec  uint32  `gorm:"column:retry_interval_seconds;not null;default:5"`
	StartedAt         *time.Time
	FinishedAt        *time.Time
	NextRetryAt       *time.Time `gorm:"index:idx_preview_conversion_status,priority:2"`
	LastErrorMessage  *string    `gorm:"size:1024"`
	PreviewObjectURL  *string    `gorm:"column:preview_object_url;size:1024"`
	ExpiresAt         time.Time  `gorm:"not null;index:idx_preview_conversion_expires"`
	CreatedAt         time.Time  `gorm:"index:idx_preview_conversion_file,priority:2"`
	UpdatedAt         time.Time
}

func (PreviewConversionTask) TableName() string { return "preview_conversion_tasks" }
