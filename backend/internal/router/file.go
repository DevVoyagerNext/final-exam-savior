package router

import (
	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/controller"
)

func RegisterFileRoutes(r *gin.RouterGroup, c *controller.Controller, authRequired gin.HandlerFunc) {
	r.GET("/file-categories", authRequired, c.ListCategories)
	r.GET("/files", authRequired, c.ListFiles)
	r.GET("/files/:fileId", authRequired, c.FileDetail)
	r.GET("/files/:fileId/preview-source", authRequired, c.PreviewSource)
	r.GET("/files/:fileId/view-source", authRequired, c.ViewSource)
	r.GET("/files/:fileId/preview-result", authRequired, c.PreviewResult)
	r.GET("/files/:fileId/view-result", authRequired, c.ViewResultHTML)
	r.GET("/files/:fileId/download-source", authRequired, c.DownloadSource)
	r.GET("/files/:fileId/download-result", authRequired, c.DownloadResult)
}
