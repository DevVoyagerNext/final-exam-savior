package model

import "time"

type FileGenerateRecord struct {
	ID              uint64 `gorm:"primaryKey;autoIncrement"`
	FileID          uint64 `gorm:"not null;uniqueIndex:uk_file_generate_records_file_id"`
	TotalStatus     string `gorm:"size:32;not null;index:idx_file_generate_records_total_status"`
	LastGeneratedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (FileGenerateRecord) TableName() string { return "file_generate_records" }
