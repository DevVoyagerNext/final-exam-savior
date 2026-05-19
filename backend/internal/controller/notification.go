package controller

import (
	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/utils"
)

func (ctl *Controller) ListNotifications(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	var req request.ListNotificationRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := ctl.svc.ListNotifications(c.Request.Context(), current, req)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) NotificationDetail(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "notificationId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := ctl.svc.GetNotification(c.Request.Context(), current, id)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) MarkNotificationRead(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "notificationId")
	if err != nil {
		c.Error(err)
		return
	}
	if err := ctl.svc.MarkNotificationRead(c.Request.Context(), current, id); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
func (ctl *Controller) MarkNotificationReadBatch(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	var req struct {
		NotificationIDs []uint64 `json:"notificationIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	if err := ctl.svc.MarkNotificationsReadBatch(c.Request.Context(), current, req.NotificationIDs); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
func (ctl *Controller) UnreadCount(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	data, err := ctl.svc.UnreadCount(c.Request.Context(), current)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
