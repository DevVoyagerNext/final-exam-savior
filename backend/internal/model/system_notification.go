package model

import "time"

type SystemNotification struct {
	ID                 uint64  `gorm:"primaryKey;autoIncrement"`
	UserID             uint64  `gorm:"not null;index:idx_notifications_user_status,priority:1;index:idx_notifications_user_type,priority:1"`
	Type               string  `gorm:"size:32;not null;index:idx_notifications_user_type,priority:2"`
	Title              string  `gorm:"size:255;not null"`
	Summary            string  `gorm:"size:512;not null"`
	Content            *string `gorm:"type:text"`
	Status             string  `gorm:"size:32;not null;index:idx_notifications_user_status,priority:2"`
	TargetType         string  `gorm:"size:32;not null"`
	TargetID           *uint64
	TargetSnapshotName *string `gorm:"size:255"`
	ErrorSummary       *string `gorm:"size:1024"`
	MergedKey          *string `gorm:"size:128;index:idx_notifications_merged_key"`
	ReadAt             *time.Time
	ExpiresAt          time.Time `gorm:"not null;index:idx_notifications_expires"`
	CreatedAt          time.Time `gorm:"index:idx_notifications_user_status,priority:3;index:idx_notifications_user_type,priority:3"`
	UpdatedAt          time.Time
}

func (SystemNotification) TableName() string { return "system_notifications" }
