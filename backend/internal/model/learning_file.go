package model

import "time"

type LearningFile struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement"`
	SourceFileHash  string    `gorm:"size:128;not null;uniqueIndex:uk_learning_files_hash"`
	SourceFileName  string    `gorm:"size:255;not null;index:idx_learning_files_name"`
	SourceFileType  string    `gorm:"size:128;not null"`
	SourceFileSize  uint64    `gorm:"not null"`
	SourceObjectURL string    `gorm:"column:source_object_url;size:1024;not null"`
	CategoryID      uint64    `gorm:"not null;index:idx_learning_files_category"`
	Visibility      string    `gorm:"size:32;not null;index:idx_learning_files_visibility"`
	UploadUserID    uint64    `gorm:"not null;index:idx_learning_files_upload_user"`
	UploadTime      time.Time `gorm:"not null;index:idx_learning_files_upload_time"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (LearningFile) TableName() string { return "learning_files" }
