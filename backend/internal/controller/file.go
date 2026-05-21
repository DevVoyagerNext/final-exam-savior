package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/utils"
)

func (ctl *Controller) ListFiles(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	var req request.ListFileRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := ctl.svc.ListFiles(c.Request.Context(), current, req, false)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) ListAdminFiles(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	var req request.ListFileRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := ctl.svc.ListFiles(c.Request.Context(), current, req, true)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) FileDetail(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := ctl.svc.GetFileDetail(c.Request.Context(), current, id)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) UploadFile(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
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
	data, err := ctl.svc.UploadFile(c.Request.Context(), current, fileHeader, categoryID, visibility)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) DeleteFile(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "fileId")
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
	if err := ctl.svc.DeleteFile(c.Request.Context(), current, id, req.ConfirmText); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
func (ctl *Controller) PreviewSource(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := ctl.svc.PreviewSource(c.Request.Context(), current, id)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) PreviewResult(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	itemType := c.Query("itemType")
	data, err := ctl.svc.PreviewResult(c.Request.Context(), current, id, itemType)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) ViewResultHTML(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	body, err := ctl.svc.ViewResultHTML(c.Request.Context(), current, id, c.Query("itemType"))
	if err != nil {
		c.Error(err)
		return
	}
	c.Data(200, "text/html; charset=utf-8", body)
}
func (ctl *Controller) ViewSource(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	body, contentType, fileName, err := ctl.svc.ViewSourcePDF(c.Request.Context(), current, id)
	if err != nil {
		c.Error(err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", fileName))
	c.Data(http.StatusOK, contentType, body)
}
func (ctl *Controller) DownloadSource(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := ctl.svc.DownloadSource(c.Request.Context(), current, id)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) DownloadResult(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "fileId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := ctl.svc.DownloadResult(c.Request.Context(), current, id, c.Query("itemType"))
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) ServeLocalStorage(c *gin.Context) {
	fullPath, err := ctl.svc.ResolveLocalObjectPath(c.Param("objectKey"), c.Query("exp"), c.Query("sig"))
	if err != nil {
		c.Error(err)
		return
	}
	c.File(fullPath)
}
