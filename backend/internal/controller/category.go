package controller

import (
	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/utils"
)

func (ctl *Controller) ListCategories(c *gin.Context) {
	data, err := ctl.svc.ListCategories(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) CreateCategory(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	var req request.CategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := ctl.svc.CreateCategory(c.Request.Context(), current, req); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
func (ctl *Controller) UpdateCategory(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "categoryId")
	if err != nil {
		c.Error(err)
		return
	}
	var req request.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := ctl.svc.UpdateCategory(c.Request.Context(), current, id, req); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
func (ctl *Controller) DeleteCategory(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "categoryId")
	if err != nil {
		c.Error(err)
		return
	}
	if err := ctl.svc.DeleteCategory(c.Request.Context(), current, id); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
