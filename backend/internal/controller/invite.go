package controller

import (
	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/utils"
)

func (ctl *Controller) CreateInviteCode(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	var req request.CreateInviteCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := ctl.svc.CreateInviteCode(c.Request.Context(), current, req); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
func (ctl *Controller) BatchGenerateInviteCodes(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	var req request.BatchGenerateInviteCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := ctl.svc.BatchGenerateInviteCodes(c.Request.Context(), current, req)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) ListInviteCodes(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	var req request.ListInviteCodeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := ctl.svc.ListInviteCodes(c.Request.Context(), current, req)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) UpdateInviteRemark(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "inviteCodeId")
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
	if err := ctl.svc.UpdateInviteCodeRemark(c.Request.Context(), current, id, req.Remark); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
func (ctl *Controller) DeleteInviteCode(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "inviteCodeId")
	if err != nil {
		c.Error(err)
		return
	}
	if err := ctl.svc.DeleteInviteCode(c.Request.Context(), current, id); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
