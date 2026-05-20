package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/model"
	"final-exam-savior/backend/internal/platform"
)

func (s *Service) ListFiles(ctx context.Context, current *CurrentUser, req request.ListFileRequest, adminMode bool) (map[string]any, error) {
	tx := s.dao.Gorm().WithContext(ctx).Model(&model.LearningFile{})
	if !adminMode {
		tx = tx.Where("visibility = ? OR upload_user_id = ?", "PUBLIC", current.User.ID)
	}
	if req.Keyword != "" {
		tx = tx.Where("source_file_name LIKE ?", "%"+req.Keyword+"%")
	}
	if req.CategoryID > 0 {
		tx = tx.Where("category_id = ?", req.CategoryID)
	}
	if req.Visibility != "" {
		tx = tx.Where("visibility = ?", req.Visibility)
	}
	if adminMode && req.UploadUserID > 0 {
		tx = tx.Where("upload_user_id = ?", req.UploadUserID)
	}

	var files []model.LearningFile
	page, err := pageQuery(ctx, tx, req.PageNo, req.PageSize, "upload_time DESC", &files, func(file model.LearningFile) map[string]any {
		return map[string]any{}
	})
	if err != nil {
		return nil, err
	}
	list := make([]map[string]any, 0, len(files))
	for _, file := range files {
		dto, dtoErr := s.fileListItem(ctx, file)
		if dtoErr != nil {
			return nil, dtoErr
		}
		if req.GenerateStatus != "" && dto["generateTotalStatus"] != req.GenerateStatus {
			continue
		}
		list = append(list, dto)
	}
	page["list"] = list
	page["total"] = len(list)
	return page, nil
}

func (s *Service) GetFileDetail(ctx context.Context, current *CurrentUser, fileID uint64) (map[string]any, error) {
	file, err := s.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, err
	}
	item, err := s.fileListItem(ctx, file)
	if err != nil {
		return nil, err
	}
	var preview model.FilePreviewRecord
	_ = s.dao.Gorm().WithContext(ctx).Where("file_id = ?", fileID).First(&preview).Error
	var record model.FileGenerateRecord
	_ = s.dao.Gorm().WithContext(ctx).Where("file_id = ?", fileID).First(&record).Error
	var items []model.FileGenerateRecordItem
	_ = s.dao.Gorm().WithContext(ctx).Where("generate_record_id = ?", record.ID).Find(&items).Error

	generateItems := make([]map[string]any, 0, len(items))
	for _, entry := range items {
		generateItems = append(generateItems, map[string]any{
			"itemType":        entry.ItemType,
			"itemStatus":      entry.ItemStatus,
			"resultObjectUrl": entry.ResultObjectURL,
		})
	}

	item["sourceFileHash"] = file.SourceFileHash
	item["sourceFileUrl"] = file.SourceObjectURL
	item["generateRecord"] = map[string]any{
		"totalStatus":     record.TotalStatus,
		"lastGeneratedAt": formatTimePtr(record.LastGeneratedAt),
		"items":           generateItems,
	}
	item["previewRecord"] = map[string]any{
		"previewMode":      "DIRECT",
		"previewStatus":    "SUCCESS",
		"previewObjectUrl": nil,
	}
	return item, nil
}

func (s *Service) UploadFile(ctx context.Context, current *CurrentUser, fileHeader *multipart.FileHeader, categoryID uint64, visibility string) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if fileHeader == nil {
		return nil, newError(http.StatusBadRequest, codeBadRequest, "文件不能为空", nil)
	}
	if categoryID == 0 {
		return nil, newError(http.StatusBadRequest, codeBadRequest, "categoryId 不能为空", nil)
	}
	var category model.FileCategory
	if err := s.dao.Gorm().WithContext(ctx).First(&category, categoryID).Error; err != nil {
		return nil, newError(http.StatusBadRequest, codeBusiness, "分类不存在", err)
	}
	data, err := platform.ReadMultipartFile(fileHeader)
	if err != nil {
		return nil, newError(http.StatusBadRequest, codeBadRequest, "读取上传文件失败", err)
	}
	hash := sha256HexBytes(data)
	objectKey := platform.BuildObjectKey("source", fileHeader.Filename)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	sourceURL, err := s.storage.Upload(ctx, objectKey, contentType, data)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "上传源文件到 OSS 失败", err)
	}

	var response map[string]any
	err = s.dao.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		var file model.LearningFile
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_file_hash = ?", hash).First(&file).Error
		reuse := findErr == nil
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			file = model.LearningFile{
				SourceFileHash:  hash,
				SourceFileName:  fileHeader.Filename,
				SourceFileType:  contentType,
				SourceFileSize:  uint64(len(data)),
				SourceObjectURL: sourceURL,
				CategoryID:      categoryID,
				Visibility:      visibility,
				UploadUserID:    current.User.ID,
				UploadTime:      now,
			}
			if err := tx.Create(&file).Error; err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			previewMode := detectPreviewMode(contentType)
			previewStatus := "SUCCESS"
			if previewMode == "CONVERT_TO_PDF" {
				previewStatus = "PENDING"
			}
			preview := model.FilePreviewRecord{
				FileID:        file.ID,
				PreviewMode:   previewMode,
				PreviewStatus: previewStatus,
			}
			if err := tx.Create(&preview).Error; err != nil {
				return fmt.Errorf("create preview record: %w", err)
			}
			generateRecord := model.FileGenerateRecord{
				FileID:      file.ID,
				TotalStatus: "PENDING",
			}
			if err := tx.Create(&generateRecord).Error; err != nil {
				return fmt.Errorf("create generate record: %w", err)
			}
			for _, itemType := range []string{"QUESTION", "KNOWLEDGE", "EXTENDED"} {
				item := model.FileGenerateRecordItem{
					GenerateRecordID: generateRecord.ID,
					ItemType:         itemType,
					ItemStatus:       "PENDING",
				}
				if err := tx.Create(&item).Error; err != nil {
					return fmt.Errorf("create generate record item: %w", err)
				}
			}
		} else if findErr != nil {
			return fmt.Errorf("query file by hash: %w", findErr)
		} else {
			if err := tx.Model(&file).Updates(map[string]any{
				"source_file_name":  fileHeader.Filename,
				"source_file_type":  contentType,
				"source_file_size":  uint64(len(data)),
				"source_object_url": sourceURL,
				"category_id":       categoryID,
				"visibility":        visibility,
				"upload_user_id":    current.User.ID,
				"upload_time":       now,
			}).Error; err != nil {
				return fmt.Errorf("update reused file: %w", err)
			}
		}

		taskRemark := optionalString("复用旧结果，未重新生成")
		taskStatus := "SUCCESS"
		if !reuse {
			taskRemark = nil
			taskStatus = "PENDING"
		}
		task := model.GenerateTask{
			TaskNo:           fmt.Sprintf("GEN-%s-%04d", now.Format("20060102"), now.Nanosecond()%10000),
			FileID:           &file.ID,
			UploadUserID:     current.User.ID,
			TriggerType:      "UPLOAD",
			Status:           taskStatus,
			FileSnapshotName: fileHeader.Filename,
			FileSnapshotHash: hash,
			ReuseExisting:    reuse,
			TaskRemark:       taskRemark,
			ExpiresAt:        now.Add(30 * 24 * time.Hour),
		}
		if reuse {
			task.StartedAt = &now
			task.FinishedAt = &now
		}
		if err := tx.Create(&task).Error; err != nil {
			return fmt.Errorf("create task: %w", err)
		}

		if reuse {
			var generateRecord model.FileGenerateRecord
			if err := tx.Where("file_id = ?", file.ID).First(&generateRecord).Error; err != nil {
				return fmt.Errorf("query generate record: %w", err)
			}
			if err := tx.Model(&generateRecord).Updates(map[string]any{
				"total_status":      "SUCCESS",
				"last_generated_at": now,
			}).Error; err != nil {
				return fmt.Errorf("update generate record status: %w", err)
			}
			var latestItems []model.FileGenerateRecordItem
			if err := tx.Where("generate_record_id = ?", generateRecord.ID).Find(&latestItems).Error; err != nil {
				return fmt.Errorf("query latest items: %w", err)
			}
			for _, item := range latestItems {
				taskItem := model.GenerateTaskItem{
					TaskID:            task.ID,
					ItemType:          item.ItemType,
					Status:            item.ItemStatus,
					ResultObjectURL:   item.ResultObjectURL,
					StartedAt:         &now,
					FinishedAt:        &now,
					MaxAutoRetryCount: 3,
					RetryIntervalSec:  5,
				}
				if err := tx.Create(&taskItem).Error; err != nil {
					return fmt.Errorf("create task item for reused result: %w", err)
				}
			}
		} else {
			for _, itemType := range []string{"QUESTION", "KNOWLEDGE", "EXTENDED"} {
				taskItem := model.GenerateTaskItem{
					TaskID:            task.ID,
					ItemType:          itemType,
					Status:            "PENDING",
					MaxAutoRetryCount: 3,
					RetryIntervalSec:  5,
				}
				if err := tx.Create(&taskItem).Error; err != nil {
					return fmt.Errorf("create task item: %w", err)
				}
			}
		}

		response = map[string]any{
			"fileId":           file.ID,
			"sourceFileName":   fileHeader.Filename,
			"sourceFileHash":   hash,
			"reuseExisting":    reuse,
			"generateRecordId": file.ID,
			"taskId":           task.ID,
			"taskNo":           task.TaskNo,
			"taskStatus":       task.Status,
			"taskRemark":       derefString(task.TaskRemark),
		}
		if !reuse {
			payload, payloadErr := json.Marshal(GenerateEvent{TaskID: task.ID})
			if payloadErr != nil {
				return fmt.Errorf("marshal generate event: %w", payloadErr)
			}
			if err := s.dao.Redis().XAdd(ctx, &redis.XAddArgs{
				Stream: s.cfg.Redis.GenerateStream,
				Values: map[string]any{"payload": string(payload)},
			}).Err(); err != nil {
				return fmt.Errorf("push generate task to redis stream: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, normalizeErr(err)
	}
	return response, nil
}

func (s *Service) DeleteFile(ctx context.Context, current *CurrentUser, fileID uint64, confirmText string) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if confirmText != "DELETE" {
		return newError(http.StatusBadRequest, codeBadRequest, "确认文本不正确", nil)
	}
	return normalizeErr(s.dao.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var file model.LearningFile
		if err := tx.First(&file, fileID).Error; err != nil {
			return newError(http.StatusNotFound, codeNotFound, "文件不存在", err)
		}
		var preview model.FilePreviewRecord
		_ = tx.Where("file_id = ?", fileID).First(&preview).Error
		var generateRecord model.FileGenerateRecord
		_ = tx.Where("file_id = ?", fileID).First(&generateRecord).Error
		var items []model.FileGenerateRecordItem
		_ = tx.Where("generate_record_id = ?", generateRecord.ID).Find(&items).Error

		objectURLs := []string{file.SourceObjectURL}
		if preview.PreviewObjectURL != nil {
			objectURLs = append(objectURLs, *preview.PreviewObjectURL)
		}
		for _, item := range items {
			if item.ResultObjectURL != nil {
				objectURLs = append(objectURLs, *item.ResultObjectURL)
			}
		}
		for _, objectURL := range objectURLs {
			if objectURL == "" {
				continue
			}
			if err := s.storage.Delete(ctx, objectURL); err != nil {
				return newError(http.StatusInternalServerError, codeInternal, "删除 OSS 资源失败", err)
			}
		}
		if err := tx.Delete(&model.FileGenerateRecordItem{}, "generate_record_id = ?", generateRecord.ID).Error; err != nil {
			return fmt.Errorf("delete latest generate items: %w", err)
		}
		if generateRecord.ID > 0 {
			if err := tx.Delete(&generateRecord).Error; err != nil {
				return fmt.Errorf("delete latest generate record: %w", err)
			}
		}
		if preview.ID > 0 {
			if err := tx.Delete(&preview).Error; err != nil {
				return fmt.Errorf("delete preview record: %w", err)
			}
		}
		if err := tx.Delete(&file).Error; err != nil {
			return fmt.Errorf("delete file: %w", err)
		}
		if err := tx.Model(&model.GenerateTask{}).Where("file_id = ?", fileID).
			Updates(map[string]any{"file_id": nil, "file_deleted_snapshot": true}).Error; err != nil {
			return fmt.Errorf("update task snapshots: %w", err)
		}
		if err := tx.Model(&model.PreviewConversionTask{}).Where("file_id = ?", fileID).
			Update("file_id", nil).Error; err != nil {
			return fmt.Errorf("update preview snapshots: %w", err)
		}
		return nil
	}))
}

func (s *Service) PreviewSource(ctx context.Context, current *CurrentUser, fileID uint64) (map[string]any, error) {
	file, err := s.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, err
	}
	previewURL, renderType, err := s.buildSourcePreviewURL(ctx, file)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "生成预览地址失败", err)
	}
	s.markPreviewReady(ctx, file.ID)
	return map[string]any{
		"fileId":         file.ID,
		"previewMode":    "DIRECT",
		"previewStatus":  "SUCCESS",
		"sourceFileType": file.SourceFileType,
		"previewUrl":     previewURL,
		"expireAt":       formatTime(time.Now().Add(s.cfg.App.SignedURLTTL)),
		"renderType":     renderType,
		"downloadUrl":    fmt.Sprintf("/api/v1/files/%d/download-source", file.ID),
	}, nil
}

func (s *Service) RetryPreviewConversion(ctx context.Context, current *CurrentUser, fileID uint64) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if _, err := s.loadAccessibleFile(ctx, current, fileID); err != nil {
		return newError(http.StatusNotFound, codeNotFound, "文件不存在", err)
	}
	s.markPreviewReady(ctx, fileID)
	return nil
}

func (s *Service) PreviewResult(ctx context.Context, current *CurrentUser, fileID uint64, itemType string) (map[string]any, error) {
	file, err := s.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, err
	}
	var generateRecord model.FileGenerateRecord
	if err := s.dao.Gorm().WithContext(ctx).Where("file_id = ?", file.ID).First(&generateRecord).Error; err != nil {
		return nil, newError(http.StatusNotFound, codeNotFound, "生成记录不存在", err)
	}
	var item model.FileGenerateRecordItem
	if err := s.dao.Gorm().WithContext(ctx).Where("generate_record_id = ? AND item_type = ?", generateRecord.ID, itemType).First(&item).Error; err != nil {
		return nil, newError(http.StatusNotFound, codeNotFound, "结果不存在", err)
	}
	if item.ResultObjectURL == nil {
		return map[string]any{
			"fileId":     file.ID,
			"itemType":   item.ItemType,
			"itemStatus": item.ItemStatus,
			"previewUrl": nil,
			"expireAt":   nil,
		}, nil
	}
	signed, err := s.storage.SignGetURL(ctx, *item.ResultObjectURL, s.cfg.App.SignedURLTTL)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "生成结果预览地址失败", err)
	}
	return map[string]any{
		"fileId":     file.ID,
		"itemType":   item.ItemType,
		"itemStatus": item.ItemStatus,
		"previewUrl": signed,
		"expireAt":   formatTime(time.Now().Add(s.cfg.App.SignedURLTTL)),
	}, nil
}

func (s *Service) ViewResultHTML(ctx context.Context, current *CurrentUser, fileID uint64, itemType string) ([]byte, error) {
	file, err := s.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, err
	}
	var generateRecord model.FileGenerateRecord
	if err := s.dao.Gorm().WithContext(ctx).Where("file_id = ?", file.ID).First(&generateRecord).Error; err != nil {
		return nil, newError(http.StatusNotFound, codeNotFound, "生成记录不存在", err)
	}
	var item model.FileGenerateRecordItem
	if err := s.dao.Gorm().WithContext(ctx).Where("generate_record_id = ? AND item_type = ?", generateRecord.ID, itemType).First(&item).Error; err != nil {
		return nil, newError(http.StatusNotFound, codeNotFound, "结果不存在", err)
	}
	if item.ResultObjectURL == nil {
		return nil, newError(http.StatusConflict, codeConflict, "HTML 结果尚未生成", nil)
	}
	signed, err := s.storage.SignGetURL(ctx, *item.ResultObjectURL, s.cfg.App.SignedURLTTL)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "生成结果访问地址失败", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signed, nil)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "创建结果读取请求失败", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "读取 HTML 结果失败", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, newError(http.StatusBadGateway, codeInternal, "HTML 结果文件读取失败", fmt.Errorf("unexpected status: %d", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "读取 HTML 结果内容失败", err)
	}
	return body, nil
}

func (s *Service) DownloadSource(ctx context.Context, current *CurrentUser, fileID uint64) (map[string]any, error) {
	file, err := s.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, err
	}
	signed, err := s.storage.SignGetURL(ctx, file.SourceObjectURL, s.cfg.App.SignedURLTTL)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "生成下载地址失败", err)
	}
	return map[string]any{
		"url":      signed,
		"expireAt": formatTime(time.Now().Add(s.cfg.App.SignedURLTTL)),
	}, nil
}

func (s *Service) buildSourcePreviewURL(ctx context.Context, file model.LearningFile) (string, string, error) {
	signed, err := s.storage.SignGetURL(ctx, file.SourceObjectURL, s.cfg.App.SignedURLTTL)
	if err != nil {
		return "", "", err
	}
	if isMarkdownPreviewType(file.SourceFileType, file.SourceFileName) {
		return "https://markdown-viewer-one.vercel.app/?url=" + url.QueryEscape(signed), "MARKDOWN_RENDER", nil
	}
	if isOfficePreviewType(file.SourceFileType, file.SourceFileName) {
		return "https://view.officeapps.live.com/op/view.aspx?src=" + url.QueryEscape(signed), "PDF_SCROLL", nil
	}
	if isGoogleViewerType(file.SourceFileType, file.SourceFileName) {
		return "https://docs.google.com/gview?embedded=true&url=" + url.QueryEscape(signed), "PDF_SCROLL", nil
	}
	return signed, detectRenderType(file.SourceFileType), nil
}

func (s *Service) markPreviewReady(ctx context.Context, fileID uint64) {
	_ = s.dao.Gorm().WithContext(ctx).Model(&model.FilePreviewRecord{}).Where("file_id = ?", fileID).Updates(map[string]any{
		"preview_mode":       "DIRECT",
		"preview_status":     "SUCCESS",
		"preview_object_url": nil,
		"last_error_message": nil,
		"last_success_at":    time.Now(),
	}).Error
}

func (s *Service) DownloadResult(ctx context.Context, current *CurrentUser, fileID uint64, itemType string) (map[string]any, error) {
	result, err := s.PreviewResult(ctx, current, fileID, itemType)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"url":      result["previewUrl"],
		"expireAt": result["expireAt"],
	}, nil
}

func (s *Service) loadAccessibleFile(ctx context.Context, current *CurrentUser, fileID uint64) (model.LearningFile, error) {
	var file model.LearningFile
	if err := s.dao.Gorm().WithContext(ctx).First(&file, fileID).Error; err != nil {
		return model.LearningFile{}, newError(http.StatusNotFound, codeNotFound, "文件不存在", err)
	}
	if current.User.Role != "ADMIN" && !(file.Visibility == "PUBLIC" || file.UploadUserID == current.User.ID) {
		return model.LearningFile{}, newError(http.StatusForbidden, codeForbidden, "无权限访问该文件", nil)
	}
	return file, nil
}

func (s *Service) fileListItem(ctx context.Context, file model.LearningFile) (map[string]any, error) {
	var category model.FileCategory
	if err := s.dao.Gorm().WithContext(ctx).First(&category, file.CategoryID).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询分类失败", err)
	}
	var user model.User
	if err := s.dao.Gorm().WithContext(ctx).First(&user, file.UploadUserID).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询上传者失败", err)
	}
	var generateRecord model.FileGenerateRecord
	if err := s.dao.Gorm().WithContext(ctx).Where("file_id = ?", file.ID).First(&generateRecord).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询生成记录失败", err)
	}
	return map[string]any{
		"id":                  file.ID,
		"sourceFileName":      file.SourceFileName,
		"sourceFileType":      file.SourceFileType,
		"sourceFileSize":      file.SourceFileSize,
		"categoryId":          file.CategoryID,
		"categoryName":        category.Name,
		"visibility":          file.Visibility,
		"uploadUserId":        file.UploadUserID,
		"uploadUserEmail":     user.Email,
		"uploadTime":          formatTime(file.UploadTime),
		"generateTotalStatus": generateRecord.TotalStatus,
	}, nil
}
