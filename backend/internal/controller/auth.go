package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/platform"
	"final-exam-savior/backend/internal/utils"
)

func (ctl *Controller) SendRegisterCode(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
		platform.CaptchaPayload
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := ctl.svc.SendRegisterCode(c.Request.Context(), req.Email, req.CaptchaPayload)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) Register(c *gin.Context) {
	var req request.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := ctl.svc.Register(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) SendResetCode(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
		platform.CaptchaPayload
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := ctl.svc.SendResetCode(c.Request.Context(), req.Email, req.CaptchaPayload); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
func (ctl *Controller) ResetPassword(c *gin.Context) {
	var req request.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := ctl.svc.ResetPassword(c.Request.Context(), req); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
func (ctl *Controller) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := ctl.svc.Login(c.Request.Context(), req, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) RefreshToken(c *gin.Context) {
	var req request.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := ctl.svc.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) Me(c *gin.Context) {
	current, ok := utils.ContextCurrentUser(c)
	if !ok {
		ctl.abort(c, http.StatusUnauthorized, 40101, "未登录或登录态失效", nil)
		return
	}
	ctl.ok(c, ctl.svc.Me(c.Request.Context(), current))
}
func (ctl *Controller) Logout(c *gin.Context) {
	current, ok := utils.ContextCurrentUser(c)
	if !ok {
		ctl.abort(c, http.StatusUnauthorized, 40101, "未登录或登录态失效", nil)
		return
	}
	if err := ctl.svc.Logout(c.Request.Context(), current); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
func (ctl *Controller) ChangePassword(c *gin.Context) {
	current, ok := utils.ContextCurrentUser(c)
	if !ok {
		ctl.abort(c, http.StatusUnauthorized, 40101, "未登录或登录态失效", nil)
		return
	}
	var req request.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := ctl.svc.ChangePassword(c.Request.Context(), current, req); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
