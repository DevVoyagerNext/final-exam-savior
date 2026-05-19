package controller

import (
	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/utils"
)

func (ctl *Controller) ListUsers(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	var req request.ListUserRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := ctl.svc.ListUsers(c.Request.Context(), current, req)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) EnableUser(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "userId")
	if err != nil {
		c.Error(err)
		return
	}
	if err := ctl.svc.EnableUser(c.Request.Context(), current, id); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
func (ctl *Controller) DisableUser(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "userId")
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
	if err := ctl.svc.DisableUser(c.Request.Context(), current, id, req.Remark); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
