package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/model"
	"final-exam-savior/backend/internal/platform"
)

// StartWorkers 启动后台任务工作协程
func (s *Service) StartWorkers(ctx context.Context) {
	go s.consumeGenerateStream(ctx) // 启动 AI 内容生成任务消费者
	go s.consumePreviewStream(ctx)  // 启动预览转换任务消费者
}

// consumeGenerateStream 监听并消费 AI 内容生成任务队列 (Redis Stream)
func (s *Service) consumeGenerateStream(ctx context.Context) {
	for ctx.Err() == nil {
		// 从 Redis Stream 中读取新消息
		streams, err := s.dao.Redis().XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    s.cfg.Redis.ConsumerGroup,
			Consumer: s.cfg.Redis.ConsumerName,
			Streams:  []string{s.cfg.Redis.GenerateStream, ">"},
			Count:    1,
			Block:    s.cfg.Redis.BlockTimeout,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || strings.Contains(err.Error(), "context canceled") {
				continue
			}
			time.Sleep(time.Second)
			continue
		}
		// 处理读取到的消息
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				if err := s.handleGenerateMessage(ctx, msg); err == nil {
					// 处理成功，发送 ACK 确认消息已消费
					_ = s.dao.Redis().XAck(ctx, s.cfg.Redis.GenerateStream, s.cfg.Redis.ConsumerGroup, msg.ID).Err()
				}
			}
		}
	}
}

// consumePreviewStream 监听并消费文件预览转换任务队列 (Redis Stream)
func (s *Service) consumePreviewStream(ctx context.Context) {
	for ctx.Err() == nil {
		streams, err := s.dao.Redis().XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    s.cfg.Redis.ConsumerGroup,
			Consumer: s.cfg.Redis.ConsumerName,
			Streams:  []string{s.cfg.Redis.PreviewStream, ">"},
			Count:    1,
			Block:    s.cfg.Redis.BlockTimeout,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || strings.Contains(err.Error(), "context canceled") {
				continue
			}
			time.Sleep(time.Second)
			continue
		}
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				if err := s.handlePreviewMessage(ctx, msg); err == nil {
					_ = s.dao.Redis().XAck(ctx, s.cfg.Redis.PreviewStream, s.cfg.Redis.ConsumerGroup, msg.ID).Err()
				}
			}
		}
	}
}

// handleGenerateMessage 解析并分发 AI 生成任务消息
func (s *Service) handleGenerateMessage(ctx context.Context, msg redis.XMessage) error {
	raw, ok := msg.Values["payload"].(string)
	if !ok {
		return fmt.Errorf("generate payload missing")
	}
	var event GenerateEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return fmt.Errorf("unmarshal generate event: %w", err)
	}
	return s.processGenerateTask(ctx, event.TaskID)
}

// handlePreviewMessage 解析并分发预览转换任务消息
func (s *Service) handlePreviewMessage(ctx context.Context, msg redis.XMessage) error {
	raw, ok := msg.Values["payload"].(string)
	if !ok {
		return fmt.Errorf("preview payload missing")
	}
	var event PreviewEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return fmt.Errorf("unmarshal preview event: %w", err)
	}
	return s.processPreviewTask(ctx, event.ConversionTaskID)
}

// ListTasks 查询当前用户的任务列表（仅限管理员或任务所有者）
func (s *Service) ListTasks(ctx context.Context, current *CurrentUser, req request.ListTaskRequest) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	tx := s.dao.Gorm().WithContext(ctx).Model(&model.GenerateTask{}).Where("upload_user_id = ?", current.User.ID)
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	var tasks []model.GenerateTask
	return pageQuery(ctx, tx, req.PageNo, req.PageSize, "created_at DESC", &tasks, func(task model.GenerateTask) map[string]any {
		return map[string]any{
			"id":                  task.ID,
			"taskNo":              task.TaskNo,
			"status":              task.Status,
			"triggerType":         task.TriggerType,
			"fileSnapshotName":    task.FileSnapshotName,
			"fileDeletedSnapshot": task.FileDeletedSnapshot,
			"startedAt":           formatTimePtr(task.StartedAt),
			"finishedAt":          formatTimePtr(task.FinishedAt),
			"lastErrorMessage":    derefString(task.LastErrorMessage),
			"reuseExisting":       task.ReuseExisting,
			"taskRemark":          derefString(task.TaskRemark),
		}
	})
}

// GetTask 获取特定任务的详细信息及其子项进度
func (s *Service) GetTask(ctx context.Context, current *CurrentUser, taskID uint64) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	var task model.GenerateTask
	if err := s.dao.Gorm().WithContext(ctx).Where("id = ? AND upload_user_id = ?", taskID, current.User.ID).First(&task).Error; err != nil {
		return nil, newError(http.StatusNotFound, codeNotFound, "任务不存在", err)
	}
	var items []model.GenerateTaskItem
	if err := s.dao.Gorm().WithContext(ctx).Where("task_id = ?", task.ID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询任务子项失败", err)
	}
	dtoItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		dtoItems = append(dtoItems, map[string]any{
			"id":               item.ID,
			"itemType":         item.ItemType,
			"status":           item.Status,
			"autoRetryCount":   item.AutoRetryCount,
			"manualRetryCount": item.ManualRetryCount,
			"lastErrorMessage": derefString(item.LastErrorMessage),
			"resultObjectUrl":  item.ResultObjectURL,
		})
	}
	return map[string]any{
		"id":                  task.ID,
		"taskNo":              task.TaskNo,
		"status":              task.Status,
		"triggerType":         task.TriggerType,
		"fileSnapshotName":    task.FileSnapshotName,
		"fileDeletedSnapshot": task.FileDeletedSnapshot,
		"startedAt":           formatTimePtr(task.StartedAt),
		"finishedAt":          formatTimePtr(task.FinishedAt),
		"lastErrorMessage":    derefString(task.LastErrorMessage),
		"reuseExisting":       task.ReuseExisting,
		"taskRemark":          derefString(task.TaskRemark),
		"items":               dtoItems,
	}, nil
}

// RetryTaskItem 手动重试失败的任务子项
func (s *Service) RetryTaskItem(ctx context.Context, current *CurrentUser, taskID uint64, taskItemID uint64) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	return normalizeErr(s.dao.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 校验任务所属权
		var task model.GenerateTask
		if err := tx.Where("id = ? AND upload_user_id = ?", taskID, current.User.ID).First(&task).Error; err != nil {
			return newError(http.StatusNotFound, codeNotFound, "任务不存在", err)
		}
		// 校验子项是否存在
		var item model.GenerateTaskItem
		if err := tx.Where("id = ? AND task_id = ?", taskItemID, taskID).First(&item).Error; err != nil {
			return newError(http.StatusNotFound, codeNotFound, "任务子项不存在", err)
		}
		// 只有失败的子项才允许重试
		if item.Status != "FAIL" {
			return newError(http.StatusConflict, codeConflict, "仅失败子任务允许重试", nil)
		}
		// 重置状态为待处理
		if err := tx.Model(&item).Updates(map[string]any{
			"status":             "PENDING",
			"manual_retry_count": item.ManualRetryCount + 1,
			"last_error_message": nil,
			"last_error_code":    nil,
		}).Error; err != nil {
			return fmt.Errorf("reset task item: %w", err)
		}
		// 记录重试日志
		log := model.TaskRetryLog{
			BizType:       "GENERATE_ITEM",
			BizID:         item.ID,
			TaskID:        &taskID,
			RetryMode:     "MANUAL",
			RetryNo:       item.ManualRetryCount + 1,
			StatusBefore:  "FAIL",
			StatusAfter:   "PENDING",
			TriggerUserID: &current.User.ID,
		}
		if err := tx.Create(&log).Error; err != nil {
			return fmt.Errorf("create retry log: %w", err)
		}
		// 将任务重新塞回 Redis 队列触发处理
		payload, err := json.Marshal(GenerateEvent{TaskID: taskID})
		if err != nil {
			return fmt.Errorf("marshal generate event: %w", err)
		}
		if err := s.dao.Redis().XAdd(ctx, &redis.XAddArgs{
			Stream: s.cfg.Redis.GenerateStream,
			Values: map[string]any{"payload": string(payload)},
		}).Err(); err != nil {
			return fmt.Errorf("push retry task to stream: %w", err)
		}
		return nil
	}))
}

// enqueuePreviewTask 将预览转换任务加入异步处理队列
func (s *Service) enqueuePreviewTask(ctx context.Context, userID uint64, file model.LearningFile) error {
	now := time.Now()
	return normalizeErr(s.dao.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 创建预览转换任务记录
		task := model.PreviewConversionTask{
			FileID:            &file.ID,
			RequestUserID:     userID,
			SourceFileType:    file.SourceFileType,
			Status:            "PENDING",
			MaxAutoRetryCount: 3,
			RetryIntervalSec:  5,
			ExpiresAt:         now.Add(30 * 24 * time.Hour),
		}
		if err := tx.Create(&task).Error; err != nil {
			return fmt.Errorf("create preview conversion task: %w", err)
		}
		// 发送 Redis 消息
		payload, err := json.Marshal(PreviewEvent{ConversionTaskID: task.ID})
		if err != nil {
			return fmt.Errorf("marshal preview event: %w", err)
		}
		if err := s.dao.Redis().XAdd(ctx, &redis.XAddArgs{
			Stream: s.cfg.Redis.PreviewStream,
			Values: map[string]any{"payload": string(payload)},
		}).Err(); err != nil {
			return fmt.Errorf("push preview event: %w", err)
		}
		return nil
	}))
}

// processPreviewTask 在新方案下不再做转 PDF，而是把预览状态直接切为可在线预览
func (s *Service) processPreviewTask(ctx context.Context, taskID uint64) error {
	var task model.PreviewConversionTask
	if err := s.dao.Gorm().WithContext(ctx).First(&task, taskID).Error; err != nil {
		return err
	}
	if task.FileID == nil {
		return nil
	}
	var file model.LearningFile
	if err := s.dao.Gorm().WithContext(ctx).First(&file, *task.FileID).Error; err != nil {
		return err
	}
	now := time.Now()
	_ = s.dao.Gorm().WithContext(ctx).Model(&task).Updates(map[string]any{"status": "PROCESSING", "started_at": now}).Error

	if _, _, err := s.buildSourcePreviewURL(ctx, file); err != nil {
		return s.failPreviewTask(ctx, task, fmt.Errorf("build online preview url: %w", err))
	}
	// 更新任务状态和文件预览记录
	return normalizeErr(s.dao.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		finished := time.Now()
		if err := tx.Model(&task).Updates(map[string]any{
			"status":             "SUCCESS",
			"preview_object_url": nil,
			"finished_at":        finished,
			"last_error_message": nil,
		}).Error; err != nil {
			return fmt.Errorf("update preview task success: %w", err)
		}
		if err := tx.Model(&model.FilePreviewRecord{}).Where("file_id = ?", file.ID).Updates(map[string]any{
			"preview_mode":       "DIRECT",
			"preview_status":     "SUCCESS",
			"preview_object_url": nil,
			"last_success_at":    finished,
			"last_error_message": nil,
		}).Error; err != nil {
			return fmt.Errorf("update latest preview record: %w", err)
		}
		// 发送在线预览准备完成通知
		return tx.Create(&model.SystemNotification{
			UserID:     file.UploadUserID,
			Type:       "PREVIEW_CONVERSION_SUCCESS",
			Title:      "预览已就绪",
			Summary:    fmt.Sprintf("%s 已切换为第三方在线预览", file.SourceFileName),
			Status:     "UNREAD",
			TargetType: "PREVIEW_TASK",
			TargetID:   &task.ID,
			ExpiresAt:  finished.Add(30 * 24 * time.Hour),
		}).Error
	}))
}

// failPreviewTask 标记预览转换任务为失败并记录原因
func (s *Service) failPreviewTask(ctx context.Context, task model.PreviewConversionTask, cause error) error {
	msg := cause.Error()
	now := time.Now()
	_ = s.dao.Gorm().WithContext(ctx).Model(&task).Updates(map[string]any{
		"status":             "FAIL",
		"finished_at":        now,
		"last_error_message": msg,
	}).Error
	if task.FileID != nil {
		_ = s.dao.Gorm().WithContext(ctx).Model(&model.FilePreviewRecord{}).Where("file_id = ?", *task.FileID).Updates(map[string]any{
			"preview_status":     "FAIL",
			"last_error_message": msg,
		}).Error
	}
	return cause
}

// processGenerateTask 执行 AI 内容生成任务的主流程（包含文本提取与多项并发生成）
func (s *Service) processGenerateTask(ctx context.Context, taskID uint64) error {
	var task model.GenerateTask
	if err := s.dao.Gorm().WithContext(ctx).First(&task, taskID).Error; err != nil {
		return err
	}
	if task.FileID == nil {
		return nil
	}
	var file model.LearningFile
	if err := s.dao.Gorm().WithContext(ctx).First(&file, *task.FileID).Error; err != nil {
		return err
	}
	var items []model.GenerateTaskItem
	if err := s.dao.Gorm().WithContext(ctx).Where("task_id = ?", taskID).Order("id ASC").Find(&items).Error; err != nil {
		return err
	}
	now := time.Now()
	// 标记任务和记录为处理中
	_ = s.dao.Gorm().WithContext(ctx).Model(&task).Updates(map[string]any{"status": "PROCESSING", "started_at": now}).Error
	_ = s.dao.Gorm().WithContext(ctx).Model(&model.FileGenerateRecord{}).Where("file_id = ?", file.ID).Update("total_status", "PROCESSING").Error

	// 1. 从源文件中提取纯文本内容
	sourceText, err := s.extractSourceText(ctx, file)
	if err != nil {
		return s.failGenerateTask(ctx, task, items, err)
	}

	// 2. 并发处理各个子项（题目、知识点、扩展题）
	var wg sync.WaitGroup
	errCh := make(chan error, len(items))
	for _, item := range items {
		if item.Status == "SUCCESS" {
			continue
		}
		itemCopy := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			if itemErr := s.processGenerateTaskItem(ctx, task, file, itemCopy, sourceText); itemErr != nil {
				errCh <- itemErr
			}
		}()
	}
	wg.Wait()
	close(errCh)

	// 3. 汇总错误并更新最终状态
	errList := make([]string, 0)
	for itemErr := range errCh {
		errList = append(errList, itemErr.Error())
	}
	sort.Strings(errList)

	var refreshed []model.GenerateTaskItem
	if err := s.dao.Gorm().WithContext(ctx).Where("task_id = ?", task.ID).Find(&refreshed).Error; err != nil {
		return err
	}
	status := aggregateTaskStatus(refreshed)
	finished := time.Now()
	update := map[string]any{
		"status":      status,
		"finished_at": finished,
	}
	if len(errList) > 0 {
		update["last_error_message"] = strings.Join(errList, "; ")
	}
	if err := s.dao.Gorm().WithContext(ctx).Model(&task).Updates(update).Error; err != nil {
		return err
	}
	// 同步更新文件生成记录
	_ = s.dao.Gorm().WithContext(ctx).Model(&model.FileGenerateRecord{}).Where("file_id = ?", file.ID).Updates(map[string]any{
		"total_status":      status,
		"last_generated_at": finished,
	}).Error
	lastError := ""
	if value, ok := update["last_error_message"].(string); ok {
		lastError = value
	}
	// 发送生成结果通知
	return s.notifyGenerateResult(ctx, task, file, status, lastError)
}

// processGenerateTaskItem 调用 AI 生成单个子项内容并保存
func (s *Service) processGenerateTaskItem(ctx context.Context, task model.GenerateTask, file model.LearningFile, item model.GenerateTaskItem, sourceText string) error {
	now := time.Now()
	_ = s.dao.Gorm().WithContext(ctx).Model(&item).Updates(map[string]any{"status": "PROCESSING", "started_at": now}).Error
	// 调用 AI 接口生成 HTML 内容
	html, err := s.ai.GenerateHTML(ctx, item.ItemType, sourceText)
	if err != nil {
		msg := err.Error()
		_ = s.dao.Gorm().WithContext(ctx).Model(&item).Updates(map[string]any{
			"status":             "FAIL",
			"finished_at":        time.Now(),
			"last_error_message": msg,
		}).Error
		return err
	}
	// 将生成的 HTML 上传到存储
	objectURL, err := s.storage.Upload(ctx, platform.BuildObjectKey("result", fmt.Sprintf("%d_%s.html", file.ID, strings.ToLower(item.ItemType))), "text/html; charset=utf-8", []byte(html))
	if err != nil {
		msg := err.Error()
		_ = s.dao.Gorm().WithContext(ctx).Model(&item).Updates(map[string]any{
			"status":             "FAIL",
			"finished_at":        time.Now(),
			"last_error_message": msg,
		}).Error
		return err
	}
	// 更新子项状态和关联记录
	finished := time.Now()
	if err := s.dao.Gorm().WithContext(ctx).Model(&item).Updates(map[string]any{
		"status":             "SUCCESS",
		"result_object_url":  objectURL,
		"finished_at":        finished,
		"last_error_message": nil,
	}).Error; err != nil {
		return err
	}
	var latest model.FileGenerateRecord
	if err := s.dao.Gorm().WithContext(ctx).Where("file_id = ?", file.ID).First(&latest).Error; err != nil {
		return err
	}
	if err := s.dao.Gorm().WithContext(ctx).Model(&model.FileGenerateRecordItem{}).
		Where("generate_record_id = ? AND item_type = ?", latest.ID, item.ItemType).
		Updates(map[string]any{
			"item_status":        "SUCCESS",
			"result_object_url":  objectURL,
			"last_success_at":    finished,
			"last_error_message": nil,
		}).Error; err != nil {
		return err
	}
	return nil
}

// failGenerateTask 标记内容生成任务为失败
func (s *Service) failGenerateTask(ctx context.Context, task model.GenerateTask, items []model.GenerateTaskItem, err error) error {
	msg := err.Error()
	now := time.Now()
	_ = s.dao.Gorm().WithContext(ctx).Model(&task).Updates(map[string]any{
		"status":             "FAIL",
		"finished_at":        now,
		"last_error_message": msg,
	}).Error
	for _, item := range items {
		if item.Status != "SUCCESS" {
			_ = s.dao.Gorm().WithContext(ctx).Model(&item).Updates(map[string]any{
				"status":             "FAIL",
				"finished_at":        now,
				"last_error_message": msg,
			}).Error
		}
	}
	if task.FileID != nil {
		_ = s.dao.Gorm().WithContext(ctx).Model(&model.FileGenerateRecord{}).Where("file_id = ?", *task.FileID).Update("total_status", "FAIL").Error
	}
	return err
}

// extractSourceText 从源文件中提取纯文本（支持文本、图片 OCR、文档解析）
func (s *Service) extractSourceText(ctx context.Context, file model.LearningFile) (string, error) {
	// 获取带签名的源文件 URL
	signed, err := s.storage.SignGetURL(ctx, file.SourceObjectURL, s.cfg.App.SignedURLTTL)
	if err != nil {
		return "", fmt.Errorf("sign source url: %w", err)
	}
	// 如果是纯文本类型，直接下载并读取内容
	if isPlainText(file.SourceFileType) {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, signed, nil)
		if reqErr != nil {
			return "", fmt.Errorf("build text download request: %w", reqErr)
		}
		resp, respErr := s.httpClient.Do(req)
		if respErr != nil {
			return "", fmt.Errorf("download text source: %w", respErr)
		}
		defer resp.Body.Close()
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 限制读取 2MB
		if readErr != nil {
			return "", fmt.Errorf("read text source: %w", readErr)
		}
		return string(body), nil
	}
	// 如果是图片，调用 AI 进行 OCR 识别
	if strings.HasPrefix(file.SourceFileType, "image/") {
		return s.ai.OCRText(ctx, signed)
	}
	// 其他文档（PDF/Docx 等）调用解析器提取文本
	return s.parser.ExtractText(ctx, signed, file.SourceFileType)
}

// notifyGenerateResult 根据生成结果发送站内通知
func (s *Service) notifyGenerateResult(ctx context.Context, task model.GenerateTask, file model.LearningFile, status string, lastError string) error {
	typ := "GENERATE_SUCCESS"
	title := "生成成功"
	summary := fmt.Sprintf("%s 的复习内容已全部生成完成", file.SourceFileName)
	if status == "PARTIAL_SUCCESS" {
		typ = "PARTIAL_SUCCESS"
		title = "部分生成成功"
		summary = fmt.Sprintf("%s 的部分结果已生成完成", file.SourceFileName)
	}
	if status == "FAIL" {
		typ = "GENERATE_FAIL"
		title = "生成失败"
		summary = fmt.Sprintf("%s 生成失败", file.SourceFileName)
	}
	content := summary
	if lastError != "" {
		content = summary + "\n失败原因：" + lastError
	}
	// 创建通知记录
	return s.dao.Gorm().WithContext(ctx).Create(&model.SystemNotification{
		UserID:             file.UploadUserID,
		Type:               typ,
		Title:              title,
		Summary:            summary,
		Content:            &content,
		Status:             "UNREAD",
		TargetType:         "GENERATE_TASK",
		TargetID:           &task.ID,
		TargetSnapshotName: &task.FileSnapshotName,
		ErrorSummary:       optionalString(lastError),
		ExpiresAt:          time.Now().Add(30 * 24 * time.Hour),
	}).Error
}
