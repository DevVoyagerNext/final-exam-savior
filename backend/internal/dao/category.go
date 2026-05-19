package dao

import (
	"context"
	"final-exam-savior/backend/internal/model"
)

func (d *DAO) CountCategories(ctx context.Context) (int64, error) {
	var count int64
	err := d.Gorm().WithContext(ctx).Model(&model.FileCategory{}).Count(&count).Error
	return count, err
}

func (d *DAO) CreateCategory(ctx context.Context, category *model.FileCategory) error {
	return d.Gorm().WithContext(ctx).Create(category).Error
}
