package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	case failCnt > 0:
		return "FAIL"
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
func isPlainTextFile(contentType string, fileName string) bool {
	if isPlainText(contentType) {
		return true
	}
	switch strings.ToLower(pathExt(fileName)) {
	case ".txt", ".md", ".markdown", ".json":
		return true
	default:
		return false
	}
}
func isMarkdownPreviewType(contentType string, fileName string) bool {
	ext := strings.ToLower(pathExt(fileName))
	switch ext {
	case ".md", ".markdown":
		return true
	}
	return strings.Contains(contentType, "text/markdown")
}
func isPDFPreviewType(contentType string, fileName string) bool {
	ext := strings.ToLower(pathExt(fileName))
	if ext == ".pdf" {
		return true
	}
	return strings.Contains(contentType, "application/pdf")
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
	case ".txt", ".json":
		return true
	}
	return strings.Contains(contentType, "text/plain") ||
		strings.Contains(contentType, "application/json")
}
func pathExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}

func validateExtractedSourceText(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("提取到的源文本为空")
	}
	if looksLikeSystemErrorContent(text) {
		return fmt.Errorf("提取到的内容疑似系统错误信息，已中止生成")
	}
	return nil
}

func sanitizeGeneratedHTML(raw string) (string, error) {
	html := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
	if html == "" {
		return "", fmt.Errorf("AI 返回的 HTML 为空")
	}
	if strings.HasPrefix(html, "```") {
		html = stripMarkdownCodeFence(html)
	}
	normalized := strings.TrimSpace(html)
	if !looksLikeHTMLDocument(normalized) {
		return "", fmt.Errorf("AI 返回的内容不是完整 HTML 文档")
	}
	if looksLikeSystemErrorContent(normalized) {
		return "", fmt.Errorf("AI 返回的 HTML 疑似错误分析内容，已拒绝保存")
	}
	return normalized, nil
}

func stripMarkdownCodeFence(input string) string {
	lines := strings.Split(strings.TrimSpace(input), "\n")
	if len(lines) < 3 || !strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		return input
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	if last != "```" {
		return input
	}
	return strings.Join(lines[1:len(lines)-1], "\n")
}

func looksLikeHTMLDocument(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	return strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<body") ||
		strings.HasPrefix(lower, "<!doctype html")
}

func looksLikeSystemErrorContent(input string) bool {
	lower := strings.ToLower(input)
	signatures := []string{
		"error analysis:",
		"root cause:",
		"setting up fake worker failed",
		"only urls with a scheme in:",
		"received protocol 'c:'",
		"officeparser esm loader issue",
		"at officeparser.parseoffice",
		"at getwrappederror",
		"appdata\\local\\npm-cache\\_npx",
	}
	matches := 0
	for _, signature := range signatures {
		if strings.Contains(lower, signature) {
			matches++
		}
	}
	return matches >= 2
}
