package model

import "time"

type GenerateTaskItem struct {
	ID                uint64 `gorm:"primaryKey;autoIncrement"`
	TaskID            uint64 `gorm:"not null;uniqueIndex:uk_generate_task_items_task_type,priority:1"`
	ItemType          string `gorm:"size:32;not null;uniqueIndex:uk_generate_task_items_task_type,priority:2"`
	Status            string `gorm:"size:32;not null;index:idx_generate_task_items_status,priority:1"`
	AutoRetryCount    uint32 `gorm:"not null;default:0"`
	ManualRetryCount  uint32 `gorm:"not null;default:0"`
	MaxAutoRetryCount uint32 `gorm:"not null;default:3"`
	RetryIntervalSec  uint32 `gorm:"column:retry_interval_seconds;not null;default:5"`
	StartedAt         *time.Time
	FinishedAt        *time.Time
	NextRetryAt       *time.Time `gorm:"index:idx_generate_task_items_status,priority:2"`
	LastErrorCode     *string    `gorm:"size:64"`
	LastErrorMessage  *string    `gorm:"size:1024"`
	ResultObjectURL   *string    `gorm:"column:result_object_url;size:1024"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (GenerateTaskItem) TableName() string { return "generate_task_items" }
