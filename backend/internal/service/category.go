package service

import (
	"context"
	"net/http"
	"strings"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/model"
)

func (s *Service) ListCategories(ctx context.Context) ([]map[string]any, error) {
	var list []model.FileCategory
	if err := s.dao.Gorm().WithContext(ctx).Order("sort_no ASC, id ASC").Find(&list).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询分类失败", err)
	}
	result := make([]map[string]any, 0, len(list))
	for _, item := range list {
		result = append(result, map[string]any{
			"id":        item.ID,
			"name":      item.Name,
			"sortNo":    item.SortNo,
			"status":    item.Status,
			"isDefault": item.IsBuiltin,
		})
	}
	return result, nil
}

func (s *Service) CreateCategory(ctx context.Context, current *CurrentUser, req request.CategoryRequest) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	record := model.FileCategory{
		Name:      strings.TrimSpace(req.Name),
		Status:    "ENABLED",
		SortNo:    req.SortNo,
		IsBuiltin: false,
		CreatedBy: &current.User.ID,
	}
	if record.Name == "" {
		return newError(http.StatusBadRequest, codeBadRequest, "分类名称不能为空", nil)
	}
	if err := s.dao.Gorm().WithContext(ctx).Create(&record).Error; err != nil {
		return newError(http.StatusConflict, codeBusiness, "分类创建失败，可能名称重复", err)
	}
	return nil
}

func (s *Service) UpdateCategory(ctx context.Context, current *CurrentUser, categoryID uint64, req request.UpdateCategoryRequest) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	var record model.FileCategory
	if err := s.dao.Gorm().WithContext(ctx).First(&record, categoryID).Error; err != nil {
		return newError(http.StatusNotFound, codeNotFound, "分类不存在", err)
	}
	if record.IsBuiltin && req.Name != record.Name {
		return newError(http.StatusBadRequest, codeBusiness, "默认分类不可修改名称", nil)
	}
	if record.IsBuiltin && req.Status != "" && req.Status != record.Status {
		return newError(http.StatusBadRequest, codeBusiness, "默认分类不可禁用", nil)
	}
	updates := map[string]any{
		"sort_no": req.SortNo,
	}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if err := s.dao.Gorm().WithContext(ctx).Model(&record).Updates(updates).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "更新分类失败", err)
	}
	return nil
}

func (s *Service) DeleteCategory(ctx context.Context, current *CurrentUser, categoryID uint64) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	var record model.FileCategory
	if err := s.dao.Gorm().WithContext(ctx).First(&record, categoryID).Error; err != nil {
		return newError(http.StatusNotFound, codeNotFound, "分类不存在", err)
	}
	if record.IsBuiltin {
		return newError(http.StatusBadRequest, codeBusiness, "默认分类不可删除", nil)
	}
	var count int64
	if err := s.dao.Gorm().WithContext(ctx).Model(&model.LearningFile{}).Where("category_id = ?", categoryID).Count(&count).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "查询分类文件失败", err)
	}
	if count > 0 {
		return newError(http.StatusBadRequest, codeBusiness, "分类下仍存在文件，无法删除", nil)
	}
	if err := s.dao.Gorm().WithContext(ctx).Delete(&record).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "删除分类失败", err)
	}
	return nil
}
