package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/model"
)

func (s *Service) CreateInviteCode(ctx context.Context, current *CurrentUser, req request.CreateInviteCodeRequest) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	req.CodeType = strings.ToUpper(strings.TrimSpace(req.CodeType))
	if req.TotalQuota == 0 {
		return newError(http.StatusBadRequest, codeBadRequest, "totalQuota 必须大于 0", nil)
	}
	codeValue := strings.TrimSpace(req.Code)
	if req.CodeType == "RANDOM" {
		randomCode, err := randomAlphaNum(10)
		if err != nil {
			return newError(http.StatusInternalServerError, codeInternal, "生成邀请码失败", err)
		}
		codeValue = randomCode
	}
	if codeValue == "" {
		return newError(http.StatusBadRequest, codeBadRequest, "邀请码不能为空", nil)
	}
	record := model.InviteCode{
		Code:           codeValue,
		CodeType:       req.CodeType,
		TotalQuota:     req.TotalQuota,
		RemainingQuota: req.TotalQuota,
		Status:         "ACTIVE",
		Remark:         optionalString(req.Remark),
		CreatedBy:      current.User.ID,
	}
	if err := s.dao.Gorm().WithContext(ctx).Create(&record).Error; err != nil {
		return newError(http.StatusConflict, codeBusiness, "邀请码创建失败，可能已重复", err)
	}
	return nil
}

func (s *Service) BatchGenerateInviteCodes(ctx context.Context, current *CurrentUser, req request.BatchGenerateInviteCodeRequest) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if req.GenerateCount == 0 || req.GenerateCount > 100 {
		return nil, newError(http.StatusBadRequest, codeBadRequest, "generateCount 必须在 1-100 之间", nil)
	}
	batchNo := fmt.Sprintf("INV-%s-%04d", time.Now().Format("20060102"), time.Now().Nanosecond()%10000)
	codes := make([]map[string]any, 0, req.GenerateCount)
	for i := uint32(0); i < req.GenerateCount; i++ {
		codeValue, err := randomAlphaNum(10)
		if err != nil {
			return nil, newError(http.StatusInternalServerError, codeInternal, "生成邀请码失败", err)
		}
		record := model.InviteCode{
			Code:           codeValue,
			CodeType:       "RANDOM",
			BatchNo:        optionalString(batchNo),
			TotalQuota:     req.TotalQuota,
			RemainingQuota: req.TotalQuota,
			Status:         "ACTIVE",
			Remark:         optionalString(req.Remark),
			CreatedBy:      current.User.ID,
		}
		if err := s.dao.Gorm().WithContext(ctx).Create(&record).Error; err != nil {
			return nil, newError(http.StatusConflict, codeBusiness, "批量生成邀请码失败", err)
		}
		codes = append(codes, inviteCodeDTO(record))
	}
	return map[string]any{
		"batchNo":       batchNo,
		"generateCount": req.GenerateCount,
		"codes":         codes,
	}, nil
}

func (s *Service) ListInviteCodes(ctx context.Context, current *CurrentUser, req request.ListInviteCodeRequest) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	var list []model.InviteCode
	tx := s.dao.Gorm().WithContext(ctx).Model(&model.InviteCode{})
	if req.Keyword != "" {
		tx = tx.Where("code LIKE ?", "%"+req.Keyword+"%")
	}
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	if req.BatchNo != "" {
		tx = tx.Where("batch_no = ?", req.BatchNo)
	}
	return pageQuery(ctx, tx, req.PageNo, req.PageSize, "id DESC", &list, func(item model.InviteCode) map[string]any {
		return inviteCodeDTO(item)
	})
}

func (s *Service) UpdateInviteCodeRemark(ctx context.Context, current *CurrentUser, id uint64, remark string) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if err := s.dao.Gorm().WithContext(ctx).Model(&model.InviteCode{}).Where("id = ?", id).Update("remark", optionalString(remark)).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "修改邀请码备注失败", err)
	}
	return nil
}

func (s *Service) DeleteInviteCode(ctx context.Context, current *CurrentUser, id uint64) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if err := s.dao.Gorm().WithContext(ctx).Delete(&model.InviteCode{}, id).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "删除邀请码失败", err)
	}
	return nil
}
