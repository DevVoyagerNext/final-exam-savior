package service

import (
	"context"
	"net/http"
	"time"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/model"
)

func (s *Service) ListNotifications(ctx context.Context, current *CurrentUser, req request.ListNotificationRequest) (map[string]any, error) {
	tx := s.dao.Gorm().WithContext(ctx).Model(&model.SystemNotification{}).Where("user_id = ?", current.User.ID)
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	if req.Type != "" {
		tx = tx.Where("type = ?", req.Type)
	}
	var list []model.SystemNotification
	return pageQuery(ctx, tx, req.PageNo, req.PageSize, "created_at DESC", &list, func(item model.SystemNotification) map[string]any {
		return notificationDTO(item)
	})
}

func (s *Service) GetNotification(ctx context.Context, current *CurrentUser, id uint64) (map[string]any, error) {
	var record model.SystemNotification
	if err := s.dao.Gorm().WithContext(ctx).Where("id = ? AND user_id = ?", id, current.User.ID).First(&record).Error; err != nil {
		return nil, newError(http.StatusNotFound, codeNotFound, "通知不存在", err)
	}
	now := time.Now()
	_ = s.dao.Gorm().WithContext(ctx).Model(&record).Updates(map[string]any{"status": "READ", "read_at": now}).Error
	record.Status = "READ"
	record.ReadAt = &now
	return notificationDTO(record), nil
}

func (s *Service) MarkNotificationRead(ctx context.Context, current *CurrentUser, id uint64) error {
	if err := s.dao.Gorm().WithContext(ctx).Model(&model.SystemNotification{}).
		Where("id = ? AND user_id = ?", id, current.User.ID).
		Updates(map[string]any{"status": "READ", "read_at": time.Now()}).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "标记已读失败", err)
	}
	return nil
}

func (s *Service) MarkNotificationsReadBatch(ctx context.Context, current *CurrentUser, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	if err := s.dao.Gorm().WithContext(ctx).Model(&model.SystemNotification{}).
		Where("user_id = ? AND id IN ?", current.User.ID, ids).
		Updates(map[string]any{"status": "READ", "read_at": time.Now()}).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "批量标记已读失败", err)
	}
	return nil
}

func (s *Service) UnreadCount(ctx context.Context, current *CurrentUser) (map[string]any, error) {
	var count int64
	if err := s.dao.Gorm().WithContext(ctx).Model(&model.SystemNotification{}).
		Where("user_id = ? AND status = ?", current.User.ID, "UNREAD").
		Count(&count).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询未读数量失败", err)
	}
	return map[string]any{"unreadCount": count}, nil
}
