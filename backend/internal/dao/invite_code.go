package dao

import (
	"context"

	"final-exam-savior/backend/internal/model"
)

func (d *DAO) GetInviteCodeByCode(ctx context.Context, code string) (model.InviteCode, error) {
	var invite model.InviteCode
	err := d.Gorm().WithContext(ctx).Where("code = ?", code).First(&invite).Error
	return invite, err
}

func (d *DAO) CreateInviteCode(ctx context.Context, invite *model.InviteCode) error {
	return d.Gorm().WithContext(ctx).Create(invite).Error
}

func (d *DAO) DeleteInviteCode(ctx context.Context, id uint64) error {
	return d.Gorm().WithContext(ctx).Delete(&model.InviteCode{}, id).Error
}
