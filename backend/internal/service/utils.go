package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"final-exam-savior/backend/internal/model"
)

func pageQuery[T any](ctx context.Context, tx *gorm.DB, pageNo int, pageSize int, orderBy string, target *[]T, mapper func(T) map[string]any) (map[string]any, error) {
	if pageNo <= 0 {
		pageNo = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	var total int64
	if err := tx.WithContext(ctx).Count(&total).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "统计分页数据失败", err)
	}
	if err := tx.WithContext(ctx).Order(orderBy).Offset((pageNo - 1) * pageSize).Limit(pageSize).Find(target).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询分页数据失败", err)
	}
	list := make([]map[string]any, 0, len(*target))
	for _, item := range *target {
		list = append(list, mapper(item))
	}
	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}
	return map[string]any{
		"list":       list,
		"pageNo":     pageNo,
		"pageSize":   pageSize,
		"total":      total,
		"totalPages": totalPages,
	}, nil
}
func notificationDTO(item model.SystemNotification) map[string]any {
	content := ""
	if item.Content != nil {
		content = *item.Content
	}
	var targetTaskID any
	if item.TargetID != nil && item.TargetType == "GENERATE_TASK" {
		targetTaskID = *item.TargetID
	}
	return map[string]any{
		"id":           item.ID,
		"title":        item.Title,
		"summary":      item.Summary,
		"content":      content,
		"type":         item.Type,
		"status":       item.Status,
		"createdAt":    formatTime(item.CreatedAt),
		"targetTaskId": targetTaskID,
	}
}
func inviteCodeDTO(item model.InviteCode) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"code":           item.Code,
		"totalQuota":     item.TotalQuota,
		"remainingQuota": item.RemainingQuota,
		"remark":         derefString(item.Remark),
		"batchNo":        item.BatchNo,
		"status":         item.Status,
	}
}
func aggregateTaskStatus(items []model.GenerateTaskItem) string {
	successCnt := 0
	failCnt := 0
	for _, item := range items {
		switch item.Status {
		case "SUCCESS":
			successCnt++
		case "FAIL":
			failCnt++
		}
	}
	switch {
	case successCnt == len(items):
		return "SUCCESS"
	case failCnt == len(items):
		return "FAIL"
	case successCnt > 0 && failCnt > 0:
		return "PARTIAL_SUCCESS"
	default:
		return "PROCESSING"
	}
}
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000")
}
func formatTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}
func optionalString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
func derefString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
func validateQQEmail(email string) error {
	if !strings.HasSuffix(email, "@qq.com") || !strings.Contains(email, "@") {
		return newError(http.StatusBadRequest, codeBusiness, "当前版本仅支持 QQ 邮箱", nil)
	}
	return nil
}
func validatePasswordPair(password, confirmPassword string) error {
	if len(password) < 8 {
		return newError(http.StatusBadRequest, codeBusiness, "密码至少 8 位", nil)
	}
	if password != confirmPassword {
		return newError(http.StatusBadRequest, codeBusiness, "两次输入的密码不一致", nil)
	}
	return nil
}
func randomDigits(n int) (string, error) {
	const digits = "0123456789"
	return randomStringFromSet(n, digits)
}
func randomAlphaNum(n int) (string, error) {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	return randomStringFromSet(n, chars)
}
func randomStringFromSet(n int, charset string) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = charset[int(buf[i])%len(charset)]
	}
	return string(out), nil
}
func sha256Hex(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
func sha256HexBytes(input []byte) string {
	sum := sha256.Sum256(input)
	return hex.EncodeToString(sum[:])
}
func detectPreviewMode(contentType string) string {
	return "DIRECT"
}
func detectRenderType(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "IMAGE_VERTICAL"
	case isPlainText(contentType):
		return "MARKDOWN_RENDER"
	default:
		return "PDF_SCROLL"
	}
}
func isPlainText(contentType string) bool {
	return strings.Contains(contentType, "text/plain") || strings.Contains(contentType, "text/markdown") || strings.Contains(contentType, "application/json")
}
func isMarkdownPreviewType(contentType string, fileName string) bool {
	ext := strings.ToLower(pathExt(fileName))
	switch ext {
	case ".md", ".markdown":
		return true
	}
	return strings.Contains(contentType, "text/markdown")
}
func isOfficePreviewType(contentType string, fileName string) bool {
	ext := strings.ToLower(pathExt(fileName))
	switch ext {
	case ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx":
		return true
	}
	return strings.Contains(contentType, "officedocument") ||
		strings.Contains(contentType, "msword") ||
		strings.Contains(contentType, "ms-excel") ||
		strings.Contains(contentType, "presentation")
}
func isGoogleViewerType(contentType string, fileName string) bool {
	ext := strings.ToLower(pathExt(fileName))
	switch ext {
	case ".pdf", ".txt", ".json":
		return true
	}
	return strings.Contains(contentType, "application/pdf") ||
		strings.Contains(contentType, "text/plain") ||
		strings.Contains(contentType, "application/json")
}
func pathExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}
