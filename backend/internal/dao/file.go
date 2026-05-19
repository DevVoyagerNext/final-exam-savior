package dao

import (
	"context"

	"final-exam-savior/backend/internal/model"
)

func (d *DAO) CreateFile(ctx context.Context, file *model.LearningFile) error {
	return d.Gorm().WithContext(ctx).Create(file).Error
}

func (d *DAO) GetFileByID(ctx context.Context, id uint64) (model.LearningFile, error) {
	var file model.LearningFile
	err := d.Gorm().WithContext(ctx).First(&file, id).Error
	return file, err
}
