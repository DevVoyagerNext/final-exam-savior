package httpserver

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"final-exam-savior/backend/internal/app"
	"final-exam-savior/backend/internal/platform"
)

type Server struct {
	app *app.App
}

func New(a *app.App) *Server {
	return &Server{app: a}
}

func (s *Server) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(s.requestIDMiddleware())
	r.Use(s.errorMiddleware())
	r.GET("/storage/local/*objectKey", s.serveLocalStorage)

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register/email-code/send", s.sendRegisterCode)
			auth.POST("/register", s.register)
			auth.POST("/password-reset/email-code/send", s.sendResetCode)
			auth.POST("/password-reset/confirm", s.resetPassword)
			auth.POST("/login", s.login)
			auth.GET("/me", s.authRequired(), s.me)
			auth.POST("/logout", s.authRequired(), s.logout)
			auth.POST("/password/change", s.authRequired(), s.changePassword)
		}

		v1.GET("/file-categories", s.authRequired(), s.listCategories)
		v1.GET("/files", s.authRequired(), s.listFiles)
		v1.GET("/files/:fileId", s.authRequired(), s.fileDetail)
		v1.GET("/files/:fileId/preview-source", s.authRequired(), s.previewSource)
		v1.POST("/admin/files/:fileId/preview-conversion/retry", s.authRequired(), s.retryPreviewConversion)
		v1.GET("/files/:fileId/preview-result", s.authRequired(), s.previewResult)
		v1.GET("/files/:fileId/download-source", s.authRequired(), s.downloadSource)
		v1.GET("/files/:fileId/download-result", s.authRequired(), s.downloadResult)

		v1.GET("/tasks", s.authRequired(), s.listTasks)
		v1.GET("/tasks/:taskId", s.authRequired(), s.taskDetail)
		v1.POST("/admin/tasks/:taskId/items/:taskItemId/retry", s.authRequired(), s.retryTaskItem)

		v1.GET("/notifications", s.authRequired(), s.listNotifications)
		v1.GET("/notifications/unread-count", s.authRequired(), s.unreadCount)
		v1.GET("/notifications/:notificationId", s.authRequired(), s.notificationDetail)
		v1.POST("/notifications/:notificationId/read", s.authRequired(), s.markNotificationRead)
		v1.POST("/notifications/read/batch", s.authRequired(), s.markNotificationReadBatch)

		admin := v1.Group("/admin", s.authRequired())
		{
			admin.POST("/invite-codes", s.createInviteCode)
			admin.POST("/invite-codes/batch-generate", s.batchGenerateInviteCodes)
			admin.GET("/invite-codes", s.listInviteCodes)
			admin.PUT("/invite-codes/:inviteCodeId/remark", s.updateInviteRemark)
			admin.DELETE("/invite-codes/:inviteCodeId", s.deleteInviteCode)

			admin.POST("/file-categories", s.createCategory)
			admin.PUT("/file-categories/:categoryId", s.updateCategory)
			admin.DELETE("/file-categories/:categoryId", s.deleteCategory)

			admin.POST("/files/upload", s.uploadFile)
			admin.DELETE("/files/:fileId", s.deleteFile)
			admin.GET("/files", s.listAdminFiles)

			admin.GET("/users", s.listUsers)
			admin.POST("/users/:userId/enable", s.enableUser)
			admin.POST("/users/:userId/disable", s.disableUser)
		}
	}
	return r
}

func (s *Server) requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.NewString()
		c.Set("request_id", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

func (s *Server) errorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		err := c.Errors.Last().Err
		var appErr *app.AppError
		if errors.As(err, &appErr) {
			s.abort(c, appErr.HTTPStatus, appErr.Code, appErr.Message, nil)
			return
		}
		s.abort(c, http.StatusInternalServerError, 50001, "系统异常", nil)
	}
}

func (s *Server) authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			s.abort(c, http.StatusUnauthorized, 40101, "未登录或登录态失效", nil)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		current, err := s.app.ValidateSession(c.Request.Context(), token)
		if err != nil {
			c.Error(err)
			c.Abort()
			return
		}
		c.Set("current_user", current)
		c.Next()
	}
}

func (s *Server) ok(c *gin.Context, data any) {
	requestID, _ := c.Get("request_id")
	c.JSON(http.StatusOK, gin.H{
		"code":      0,
		"message":   "ok",
		"data":      data,
		"requestId": requestID,
	})
}

func (s *Server) abort(c *gin.Context, httpStatus int, code int, message string, data any) {
	requestID, _ := c.Get("request_id")
	c.AbortWithStatusJSON(httpStatus, gin.H{
		"code":      code,
		"message":   message,
		"data":      data,
		"requestId": requestID,
	})
}

func (s *Server) sendRegisterCode(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
		platform.CaptchaPayload
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := s.app.SendRegisterCode(c.Request.Context(), req.Email, req.CaptchaPayload)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) register(c *gin.Context) {
	var req app.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := s.app.Register(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) sendResetCode(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
		platform.CaptchaPayload
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := s.app.SendResetCode(c.Request.Context(), req.Email, req.CaptchaPayload); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) resetPassword(c *gin.Context) {
	var req app.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := s.app.ResetPassword(c.Request.Context(), req); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) login(c *gin.Context) {
	var req app.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := s.app.Login(c.Request.Context(), req, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) me(c *gin.Context) {
	current, ok := app.ContextCurrentUser(c)
	if !ok {
		s.abort(c, http.StatusUnauthorized, 40101, "未登录或登录态失效", nil)
		return
	}
	s.ok(c, s.app.Me(c.Request.Context(), current))
}

func (s *Server) logout(c *gin.Context) {
	current, ok := app.ContextCurrentUser(c)
	if !ok {
		s.abort(c, http.StatusUnauthorized, 40101, "未登录或登录态失效", nil)
		return
	}
	if err := s.app.Logout(c.Request.Context(), current); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) changePassword(c *gin.Context) {
	current, ok := app.ContextCurrentUser(c)
	if !ok {
		s.abort(c, http.StatusUnauthorized, 40101, "未登录或登录态失效", nil)
		return
	}
	var req app.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := s.app.ChangePassword(c.Request.Context(), current, req); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) createInviteCode(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	var req app.CreateInviteCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := s.app.CreateInviteCode(c.Request.Context(), current, req); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) batchGenerateInviteCodes(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	var req app.BatchGenerateInviteCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := s.app.BatchGenerateInviteCodes(c.Request.Context(), current, req)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) listInviteCodes(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	var req app.ListInviteCodeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := s.app.ListInviteCodes(c.Request.Context(), current, req)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) updateInviteRemark(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "inviteCodeId")
	if err != nil {
		c.Error(err)
		return
	}
	var req struct {
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := s.app.UpdateInviteCodeRemark(c.Request.Context(), current, id, req.Remark); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) deleteInviteCode(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "inviteCodeId")
	if err != nil {
		c.Error(err)
		return
	}
	if err := s.app.DeleteInviteCode(c.Request.Context(), current, id); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) listCategories(c *gin.Context) {
	data, err := s.app.ListCategories(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) createCategory(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	var req app.CategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := s.app.CreateCategory(c.Request.Context(), current, req); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) updateCategory(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "categoryId")
	if err != nil {
		c.Error(err)
		return
	}
	var req app.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := s.app.UpdateCategory(c.Request.Context(), current, id, req); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) deleteCategory(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "categoryId")
	if err != nil {
		c.Error(err)
		return
	}
	if err := s.app.DeleteCategory(c.Request.Context(), current, id); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) listFiles(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	var req app.ListFileRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := s.app.ListFiles(c.Request.Context(), current, req, false)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) listAdminFiles(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	var req app.ListFileRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := s.app.ListFiles(c.Request.Context(), current, req, true)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) fileDetail(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := s.app.GetFileDetail(c.Request.Context(), current, id)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) uploadFile(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	var categoryID uint64
	if _, err := fmt.Sscanf(c.PostForm("categoryId"), "%d", &categoryID); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	visibility := c.PostForm("visibility")
	data, err := s.app.UploadFile(c.Request.Context(), current, fileHeader, categoryID, visibility)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) deleteFile(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	var req struct {
		ConfirmText string `json:"confirmText"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := s.app.DeleteFile(c.Request.Context(), current, id, req.ConfirmText); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) previewSource(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := s.app.PreviewSource(c.Request.Context(), current, id)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) retryPreviewConversion(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	if err := s.app.RetryPreviewConversion(c.Request.Context(), current, id); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) previewResult(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	itemType := c.Query("itemType")
	data, err := s.app.PreviewResult(c.Request.Context(), current, id, itemType)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) downloadSource(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := s.app.DownloadSource(c.Request.Context(), current, id)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) downloadResult(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := s.app.DownloadResult(c.Request.Context(), current, id, c.Query("itemType"))
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) listTasks(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	var req app.ListTaskRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := s.app.ListTasks(c.Request.Context(), current, req)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) taskDetail(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "taskId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := s.app.GetTask(c.Request.Context(), current, id)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) retryTaskItem(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	taskID, err := app.MustUint64Param(c, "taskId")
	if err != nil {
		c.Error(err)
		return
	}
	taskItemID, err := app.MustUint64Param(c, "taskItemId")
	if err != nil {
		c.Error(err)
		return
	}
	if err := s.app.RetryTaskItem(c.Request.Context(), current, taskID, taskItemID); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) listNotifications(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	var req app.ListNotificationRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := s.app.ListNotifications(c.Request.Context(), current, req)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) notificationDetail(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "notificationId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := s.app.GetNotification(c.Request.Context(), current, id)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) markNotificationRead(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "notificationId")
	if err != nil {
		c.Error(err)
		return
	}
	if err := s.app.MarkNotificationRead(c.Request.Context(), current, id); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) markNotificationReadBatch(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	var req struct {
		NotificationIDs []uint64 `json:"notificationIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := s.app.MarkNotificationsReadBatch(c.Request.Context(), current, req.NotificationIDs); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) unreadCount(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	data, err := s.app.UnreadCount(c.Request.Context(), current)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) listUsers(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	var req app.ListUserRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := s.app.ListUsers(c.Request.Context(), current, req)
	if err != nil {
		c.Error(err)
		return
	}
	s.ok(c, data)
}

func (s *Server) enableUser(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "userId")
	if err != nil {
		c.Error(err)
		return
	}
	if err := s.app.EnableUser(c.Request.Context(), current, id); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func (s *Server) disableUser(c *gin.Context) {
	current, _ := app.ContextCurrentUser(c)
	id, err := app.MustUint64Param(c, "userId")
	if err != nil {
		c.Error(err)
		return
	}
	var req struct {
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := s.app.DisableUser(c.Request.Context(), current, id, req.Remark); err != nil {
		c.Error(err)
		return
	}
	s.ok(c, nil)
}

func appErrBadRequest(err error) error {
	return &app.AppError{
		HTTPStatus: http.StatusBadRequest,
		Code:       40001,
		Message:    "参数错误",
		Err:        err,
	}
}

func (s *Server) serveLocalStorage(c *gin.Context) {
	fullPath, err := s.app.ResolveLocalObjectPath(c.Param("objectKey"), c.Query("exp"), c.Query("sig"))
	if err != nil {
		c.Error(err)
		return
	}
	c.File(fullPath)
}
