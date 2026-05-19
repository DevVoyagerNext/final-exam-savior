package model

import "time"

type FilePreviewRecord struct {
	ID               uint64  `gorm:"primaryKey;autoIncrement"`
	FileID           uint64  `gorm:"not null;uniqueIndex:uk_file_preview_records_file_id"`
	PreviewMode      string  `gorm:"size:32;not null"`
	PreviewStatus    string  `gorm:"size:32;not null;index:idx_file_preview_records_status"`
	PreviewObjectURL *string `gorm:"column:preview_object_url;size:1024"`
	LastSuccessAt    *time.Time
	LastErrorMessage *string `gorm:"size:1024"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (FilePreviewRecord) TableName() string { return "file_preview_records" }
