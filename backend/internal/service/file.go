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

// ListFiles 分页查询文件列表，支持关键词、分类、可见性、上传者及生成状态过滤
func (s *Service) ListFiles(ctx context.Context, current *CurrentUser, req request.ListFileRequest, adminMode bool) (map[string]any, error) {
	tx := s.dao.Gorm().WithContext(ctx).Model(&model.LearningFile{})
	// 非管理员只能看到公开文件或自己上传的文件
	if !adminMode {
		tx = tx.Where("visibility = ? OR upload_user_id = ?", "PUBLIC", current.User.ID)
	}
	// 关键词过滤（匹配原文件名）
	if req.Keyword != "" {
		tx = tx.Where("source_file_name LIKE ?", "%"+req.Keyword+"%")
	}
	// 分类过滤
	if req.CategoryID > 0 {
		tx = tx.Where("category_id = ?", req.CategoryID)
	}
	// 可见性过滤
	if req.Visibility != "" {
		tx = tx.Where("visibility = ?", req.Visibility)
	}
	// 管理员模式下的上传者过滤
	if adminMode && req.UploadUserID > 0 {
		tx = tx.Where("upload_user_id = ?", req.UploadUserID)
	}

	var files []model.LearningFile
	// 执行分页查询
	page, err := pageQuery(ctx, tx, req.PageNo, req.PageSize, "upload_time DESC", &files, func(file model.LearningFile) map[string]any {
		return map[string]any{}
	})
	if err != nil {
		return nil, err
	}
	// 构建 DTO 列表并按需过滤生成状态
	list := make([]map[string]any, 0, len(files))
	for _, file := range files {
		dto, dtoErr := s.fileListItem(ctx, file)
		if dtoErr != nil {
			return nil, dtoErr
		}
		// 后置过滤：生成状态
		if req.GenerateStatus != "" && dto["generateTotalStatus"] != req.GenerateStatus {
			continue
		}
		list = append(list, dto)
	}
	page["list"] = list
	page["total"] = len(list)
	return page, nil
}

// GetFileDetail 获取单个文件的详细信息，包括生成记录和预览状态
func (s *Service) GetFileDetail(ctx context.Context, current *CurrentUser, fileID uint64) (map[string]any, error) {
	// 加载并校验文件访问权限
	file, err := s.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, err
	}
	// 获取基本列表项信息
	item, err := s.fileListItem(ctx, file)
	if err != nil {
		return nil, err
	}
	// 获取预览记录
	var preview model.FilePreviewRecord
	_ = s.dao.Gorm().WithContext(ctx).Where("file_id = ?", fileID).First(&preview).Error
	// 获取生成总记录
	var record model.FileGenerateRecord
	_ = s.dao.Gorm().WithContext(ctx).Where("file_id = ?", fileID).First(&record).Error
	// 获取具体生成项（题目、知识点等）
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

	// 补充详细字段
	item["sourceFileHash"] = file.SourceFileHash
	item["sourceFileUrl"] = file.SourceObjectURL
	item["generateRecord"] = map[string]any{
		"totalStatus":     record.TotalStatus,
		"lastGeneratedAt": formatTimePtr(record.LastGeneratedAt),
		"items":           generateItems,
	}
	// 预览状态目前统一显示为直接预览成功
	item["previewRecord"] = map[string]any{
		"previewMode":      "DIRECT",
		"previewStatus":    "SUCCESS",
		"previewObjectUrl": nil,
	}
	return item, nil
}

// UploadFile 处理文件上传，包含秒传校验、事务入库及异步生成任务初始化
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
	// 校验分类是否存在
	var category model.FileCategory
	if err := s.dao.Gorm().WithContext(ctx).First(&category, categoryID).Error; err != nil {
		return nil, newError(http.StatusBadRequest, codeBusiness, "分类不存在", err)
	}
	// 读取文件内容用于计算 Hash 和上传
	data, err := platform.ReadMultipartFile(fileHeader)
	if err != nil {
		return nil, newError(http.StatusBadRequest, codeBadRequest, "读取上传文件失败", err)
	}
	// 计算文件 SHA256 作为秒传标识
	hash := sha256HexBytes(data)
	objectKey := platform.BuildObjectKey("source", fileHeader.Filename)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	// 即使文件可能已存在，通常也先上传到存储（或者可以优化为先查库再上传）
	sourceURL, err := s.storage.Upload(ctx, objectKey, contentType, data)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "上传源文件到 OSS 失败", err)
	}

	var response map[string]any
	var pushGenerateTaskID uint64
	// 开启事务处理元数据入库
	err = s.dao.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		var file model.LearningFile
		// 根据 Hash 查找是否已存在相同文件（秒传逻辑）
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_file_hash = ?", hash).First(&file).Error
		reuse := findErr == nil
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			// 新文件流程：创建文件元数据
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
			// 初始化预览记录
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
			// 初始化生成记录
			generateRecord := model.FileGenerateRecord{
				FileID:      file.ID,
				TotalStatus: "PENDING",
			}
			if err := tx.Create(&generateRecord).Error; err != nil {
				return fmt.Errorf("create generate record: %w", err)
			}
			// 预创建生成子项
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
			// 秒传/复用流程：更新基本信息
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

		// 创建后台处理任务
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
			// 如果是复用，同步旧的生成结果到新任务项中
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
			// 如果是新任务，创建待处理的子任务项
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

		// 构建返回响应
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
		// 如果是新任务，记录需要发送消息的 TaskID
		if !reuse {
			pushGenerateTaskID = task.ID
		}
		return nil
	})
	if err != nil {
		return nil, normalizeErr(err)
	}

	// 事务提交成功后，再发送消息到 Redis Stream 触发异步生成
	if pushGenerateTaskID > 0 {
		payload, payloadErr := json.Marshal(GenerateEvent{TaskID: pushGenerateTaskID})
		if payloadErr != nil {
			return nil, fmt.Errorf("marshal generate event: %w", payloadErr)
		}
		if err := s.dao.Redis().XAdd(ctx, &redis.XAddArgs{
			Stream: s.cfg.Redis.GenerateStream,
			Values: map[string]any{"payload": string(payload)},
		}).Err(); err != nil {
			return nil, fmt.Errorf("push generate task to redis stream: %w", err)
		}
	}
	return response, nil
}

// DeleteFile 删除文件及其关联的所有存储资源和数据库记录
func (s *Service) DeleteFile(ctx context.Context, current *CurrentUser, fileID uint64, confirmText string) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	// 强制要求输入 "DELETE" 以确认删除操作
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

		// 收集所有需要从 OSS/本地存储中物理删除的 URL
		objectURLs := []string{file.SourceObjectURL}
		if preview.PreviewObjectURL != nil {
			objectURLs = append(objectURLs, *preview.PreviewObjectURL)
		}
		for _, item := range items {
			if item.ResultObjectURL != nil {
				objectURLs = append(objectURLs, *item.ResultObjectURL)
			}
		}
		// 执行物理删除
		for _, objectURL := range objectURLs {
			if objectURL == "" {
				continue
			}
			if err := s.storage.Delete(ctx, objectURL); err != nil {
				return newError(http.StatusInternalServerError, codeInternal, "删除 OSS 资源失败", err)
			}
		}
		// 删除数据库中的关联记录
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
		// 历史任务记录不删除，但解除与文件的关联并记录删除快照
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

// PreviewSource 获取源文件的预览地址（支持第三方预览器路由）
func (s *Service) PreviewSource(ctx context.Context, current *CurrentUser, fileID uint64) (map[string]any, error) {
	file, err := s.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, err
	}
	// 构建预览 URL，根据文件类型分发到不同的预览服务
	previewURL, renderType, err := s.buildSourcePreviewURL(ctx, file)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "生成预览地址失败", err)
	}
	// 标记预览状态为就绪
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

// RetryPreviewConversion 管理员手动重置预览状态
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

// PreviewResult 获取 AI 生成结果的预览地址（通常是 HTML 文件）
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
	// 如果结果尚未生成，返回空地址
	if item.ResultObjectURL == nil {
		return map[string]any{
			"fileId":     file.ID,
			"itemType":   item.ItemType,
			"itemStatus": item.ItemStatus,
			"previewUrl": nil,
			"expireAt":   nil,
		}, nil
	}
	// 为生成的 HTML 结果生成带签名的访问地址
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

// ViewResultHTML 读取并直接返回 AI 生成的 HTML 内容（用于内嵌展示）
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
	// 获取后端存储中的原始 HTML
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
	// 限制读取大小防止 OOM (4MB)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "读取 HTML 结果内容失败", err)
	}
	sanitized, err := sanitizeGeneratedHTML(string(body))
	if err != nil {
		return nil, newError(http.StatusConflict, codeConflict, "HTML 结果内容无效，请重新生成", err)
	}
	return []byte(sanitized), nil
}

// DownloadSource 获取源文件的直接下载地址
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

// ViewSourcePDF 后端中转 PDF 内容以实现 inline 预览（解决前端新标签页 401 问题）
func (s *Service) ViewSourcePDF(ctx context.Context, current *CurrentUser, fileID uint64) ([]byte, string, string, error) {
	file, err := s.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, "", "", err
	}
	signed, err := s.storage.SignGetURL(ctx, file.SourceObjectURL, s.cfg.App.SignedURLTTL)
	if err != nil {
		return nil, "", "", newError(http.StatusInternalServerError, codeInternal, "生成预览地址失败", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signed, nil)
	if err != nil {
		return nil, "", "", newError(http.StatusInternalServerError, codeInternal, "构建源文件请求失败", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", "", newError(http.StatusBadGateway, codeInternal, "拉取源文件失败", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, "", "", newError(http.StatusBadGateway, codeInternal, "源文件预览失败", fmt.Errorf("source status: %d", resp.StatusCode))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", newError(http.StatusBadGateway, codeInternal, "读取源文件失败", err)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = file.SourceFileType
	}
	if contentType == "" {
		contentType = "application/pdf"
	}
	return body, contentType, file.SourceFileName, nil
}

// buildSourcePreviewURL 根据文件类型构建预览 URL（支持微软/谷歌/Markdown 专用预览器）
func (s *Service) buildSourcePreviewURL(ctx context.Context, file model.LearningFile) (string, string, error) {
	signed, err := s.storage.SignGetURL(ctx, file.SourceObjectURL, s.cfg.App.SignedURLTTL)
	if err != nil {
		return "", "", err
	}
	// Markdown 走 Vercel 部署的专用预览器
	if isMarkdownPreviewType(file.SourceFileType, file.SourceFileName) {
		return "https://markdown-viewer-one.vercel.app/?url=" + url.QueryEscape(signed), "MARKDOWN_RENDER", nil
	}
	// PDF 走后端内嵌中转（保证鉴权）
	if isPDFPreviewType(file.SourceFileType, file.SourceFileName) {
		return fmt.Sprintf("/api/v1/files/%d/view-source", file.ID), "PDF_SCROLL", nil
	}
	// Office 文档走微软在线预览
	if isOfficePreviewType(file.SourceFileType, file.SourceFileName) {
		return "https://view.officeapps.live.com/op/view.aspx?src=" + url.QueryEscape(signed), "PDF_SCROLL", nil
	}
	// 其他受支持文档走谷歌预览
	if isGoogleViewerType(file.SourceFileType, file.SourceFileName) {
		return "https://docs.google.com/gview?embedded=true&url=" + url.QueryEscape(signed), "PDF_SCROLL", nil
	}
	// 默认直接返回带签名的源文件 URL
	return signed, detectRenderType(file.SourceFileType), nil
}

// markPreviewReady 快速更新数据库中的预览状态为成功
func (s *Service) markPreviewReady(ctx context.Context, fileID uint64) {
	_ = s.dao.Gorm().WithContext(ctx).Model(&model.FilePreviewRecord{}).Where("file_id = ?", fileID).Updates(map[string]any{
		"preview_mode":       "DIRECT",
		"preview_status":     "SUCCESS",
		"preview_object_url": nil,
		"last_error_message": nil,
		"last_success_at":    time.Now(),
	}).Error
}

// DownloadResult 获取生成结果的下载地址
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

// loadAccessibleFile 加载文件并检查当前用户是否有权访问（公开或所有者或管理员）
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

// fileListItem 辅助方法：将文件模型转换为包含分类、上传者及生成状态的 DTO
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
