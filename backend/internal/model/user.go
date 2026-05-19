package model

import "time"

type User struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	Email        string    `gorm:"size:128;not null;uniqueIndex:uk_users_email"`
	PasswordHash string    `gorm:"size:255;not null"`
	Role         string    `gorm:"size:32;not null;index:idx_users_role"`
	Status       string    `gorm:"size:32;not null;index:idx_users_status"`
	RegisteredAt time.Time `gorm:"not null;index:idx_users_registered_at"`
	LastLoginAt  *time.Time
	DisabledAt   *time.Time
	DisabledBy   *uint64
	Remark       *string `gorm:"size:255"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "users" }
