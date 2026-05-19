package dao

import (
	"context"
	"time"

	"final-exam-savior/backend/internal/model"
)

func (d *DAO) GetUserByEmail(ctx context.Context, email string) (model.User, error) {
	var user model.User
	err := d.Gorm().WithContext(ctx).Where("email = ?", email).First(&user).Error
	return user, err
}

func (d *DAO) GetUserByID(ctx context.Context, id uint64) (model.User, error) {
	var user model.User
	err := d.Gorm().WithContext(ctx).First(&user, id).Error
	return user, err
}

func (d *DAO) CreateUser(ctx context.Context, user *model.User) error {
	return d.Gorm().WithContext(ctx).Create(user).Error
}

func (d *DAO) UpdateUserLoginTime(ctx context.Context, userID uint64, loginTime time.Time) error {
	return d.Gorm().WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("last_login_at", loginTime).Error
}

func (d *DAO) GetActiveSession(ctx context.Context, token string) (model.UserSession, error) {
	var session model.UserSession
	err := d.Gorm().WithContext(ctx).
		Where("session_token = ? AND status = ? AND expires_at > ?", token, "ACTIVE", time.Now()).
		First(&session).Error
	return session, err
}

func (d *DAO) CreateSession(ctx context.Context, session *model.UserSession) error {
	return d.Gorm().WithContext(ctx).Create(session).Error
}

func (d *DAO) CheckUserExists(ctx context.Context, email string) (bool, error) {
	var exists int64
	err := d.Gorm().WithContext(ctx).Model(&model.User{}).Where("email = ?", email).Count(&exists).Error
	return exists > 0, err
}
