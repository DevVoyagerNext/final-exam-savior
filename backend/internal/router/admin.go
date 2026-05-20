package router

import (
	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/controller"
)

func RegisterAdminRoutes(r *gin.RouterGroup, c *controller.Controller, adminRequired gin.HandlerFunc) {
	admin := r.Group("/admin", adminRequired)
	{
		admin.POST("/invite-codes", c.CreateInviteCode)
		admin.POST("/invite-codes/batch-generate", c.BatchGenerateInviteCodes)
		admin.GET("/invite-codes", c.ListInviteCodes)
		admin.PUT("/invite-codes/:inviteCodeId/remark", c.UpdateInviteRemark)
		admin.DELETE("/invite-codes/:inviteCodeId", c.DeleteInviteCode)

		admin.POST("/file-categories", c.CreateCategory)
		admin.PUT("/file-categories/:categoryId", c.UpdateCategory)
		admin.DELETE("/file-categories/:categoryId", c.DeleteCategory)

		admin.POST("/files/upload", c.UploadFile)
		admin.DELETE("/files/:fileId", c.DeleteFile)
		admin.GET("/files", c.ListAdminFiles)

		admin.GET("/users", c.ListUsers)
		admin.POST("/users/:userId/enable", c.EnableUser)
		admin.POST("/users/:userId/disable", c.DisableUser)
	}
}
