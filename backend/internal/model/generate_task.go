package model

import "time"

type GenerateTask struct {
	ID                  uint64  `gorm:"primaryKey;autoIncrement"`
	TaskNo              string  `gorm:"size:64;not null;uniqueIndex:uk_generate_tasks_task_no"`
	FileID              *uint64 `gorm:"index:idx_generate_tasks_file_id"`
	UploadUserID        uint64  `gorm:"not null;index:idx_generate_tasks_upload_user,priority:1"`
	TriggerType         string  `gorm:"size:32;not null"`
	Status              string  `gorm:"size:32;not null;index:idx_generate_tasks_status,priority:1"`
	FileSnapshotName    string  `gorm:"size:255;not null"`
	FileSnapshotHash    string  `gorm:"size:128;not null"`
	FileDeletedSnapshot bool    `gorm:"not null"`
	ReuseExisting       bool    `gorm:"not null;default:false"`
	TaskRemark          *string `gorm:"size:255"`
	StartedAt           *time.Time
	FinishedAt          *time.Time
	LastErrorMessage    *string   `gorm:"size:1024"`
	ExpiresAt           time.Time `gorm:"not null;index:idx_generate_tasks_expires"`
	CreatedAt           time.Time `gorm:"index:idx_generate_tasks_upload_user,priority:2;index:idx_generate_tasks_status,priority:2"`
	UpdatedAt           time.Time
}

func (GenerateTask) TableName() string { return "generate_tasks" }
