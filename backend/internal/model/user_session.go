package model

import "time"

type UserSession struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement"`
	UserID        uint64    `gorm:"not null;index:idx_user_sessions_user_id"`
	SessionToken  string    `gorm:"size:128;not null;uniqueIndex:uk_user_sessions_token"`
	Status        string    `gorm:"size:32;not null;index:idx_user_sessions_status_expires,priority:1"`
	LoginIP       *string   `gorm:"size:64"`
	UserAgent     *string   `gorm:"size:512"`
	IssuedAt      time.Time `gorm:"not null"`
	ExpiresAt     time.Time `gorm:"not null;index:idx_user_sessions_status_expires,priority:2"`
	InvalidatedAt *time.Time
	InvalidReason *string `gorm:"size:64"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (UserSession) TableName() string { return "user_sessions" }
