package model

import "time"

type FileGenerateRecordItem struct {
	ID               uint64  `gorm:"primaryKey;autoIncrement"`
	GenerateRecordID uint64  `gorm:"not null;uniqueIndex:uk_file_generate_record_items_record_type,priority:1"`
	ItemType         string  `gorm:"size:32;not null;uniqueIndex:uk_file_generate_record_items_record_type,priority:2"`
	ItemStatus       string  `gorm:"size:32;not null;index:idx_file_generate_record_items_status"`
	ResultObjectURL  *string `gorm:"column:result_object_url;size:1024"`
	LastSuccessAt    *time.Time
	LastErrorMessage *string `gorm:"size:1024"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (FileGenerateRecordItem) TableName() string { return "file_generate_record_items" }
