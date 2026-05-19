package model

import "time"

type TaskRetryLog struct {
	ID            uint64  `gorm:"primaryKey;autoIncrement"`
	BizType       string  `gorm:"size:32;not null;index:idx_retry_logs_biz,priority:1"`
	BizID         uint64  `gorm:"not null;index:idx_retry_logs_biz,priority:2"`
	TaskID        *uint64 `gorm:"index:idx_retry_logs_task,priority:1"`
	RetryMode     string  `gorm:"size:32;not null"`
	RetryNo       uint32  `gorm:"not null"`
	StatusBefore  string  `gorm:"size:32;not null"`
	StatusAfter   string  `gorm:"size:32;not null"`
	TriggerUserID *uint64
	ErrorMessage  *string   `gorm:"size:1024"`
	CreatedAt     time.Time `gorm:"index:idx_retry_logs_biz,priority:3;index:idx_retry_logs_task,priority:2"`
}

func (TaskRetryLog) TableName() string { return "task_retry_logs" }
