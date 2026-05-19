package model

import "time"

type InviteCode struct {
	ID             uint64  `gorm:"primaryKey;autoIncrement"`
	Code           string  `gorm:"size:64;not null;uniqueIndex:uk_invite_codes_code"`
	CodeType       string  `gorm:"size:32;not null"`
	BatchNo        *string `gorm:"size:64;index:idx_invite_codes_batch_no"`
	TotalQuota     uint32  `gorm:"not null"`
	RemainingQuota uint32  `gorm:"not null"`
	Status         string  `gorm:"size:32;not null;index:idx_invite_codes_status"`
	Remark         *string `gorm:"size:255"`
	CreatedBy      uint64  `gorm:"not null;index:idx_invite_codes_created_by"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (InviteCode) TableName() string { return "invite_codes" }
