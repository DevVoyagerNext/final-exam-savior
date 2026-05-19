package model

import "time"

type FileCategory struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	Name      string `gorm:"size:64;not null;uniqueIndex:uk_file_categories_name"`
	IsBuiltin bool   `gorm:"not null"`
	Status    string `gorm:"size:32;not null;index:idx_file_categories_status_sort,priority:1"`
	SortNo    int    `gorm:"not null;default:0;index:idx_file_categories_status_sort,priority:2"`
	CreatedBy *uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (FileCategory) TableName() string { return "file_categories" }
