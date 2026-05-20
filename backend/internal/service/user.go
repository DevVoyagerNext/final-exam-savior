package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/model"
)

func (s *Service) ListUsers(ctx context.Context, current *CurrentUser, req request.ListUserRequest) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	tx := s.dao.Gorm().WithContext(ctx).Model(&model.User{})
	if req.Email != "" {
		tx = tx.Where("email LIKE ?", "%"+req.Email+"%")
	}
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	var users []model.User
	return pageQuery(ctx, tx, req.PageNo, req.PageSize, "registered_at DESC", &users, func(user model.User) map[string]any {
		return map[string]any{
			"id":           user.ID,
			"email":        user.Email,
			"role":         user.Role,
			"status":       user.Status,
			"registeredAt": formatTime(user.RegisteredAt),
		}
	})
}

func (s *Service) EnableUser(ctx context.Context, current *CurrentUser, id uint64) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	return normalizeErr(s.dao.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
			"status":      "ENABLED",
			"disabled_at": nil,
			"disabled_by": nil,
			"remark":      nil,
		}).Error; err != nil {
			return fmt.Errorf("enable user: %w", err)
		}
		return nil
	}))
}

func (s *Service) DisableUser(ctx context.Context, current *CurrentUser, id uint64, remark string) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if err := normalizeErr(s.dao.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
			"status":      "DISABLED",
			"disabled_at": now,
			"disabled_by": current.User.ID,
			"remark":      optionalString(remark),
		}).Error; err != nil {
			return fmt.Errorf("disable user: %w", err)
		}
		return nil
	})); err != nil {
		return err
	}
	return s.revokeAllRefreshTokens(ctx, id)
}
