package controller

import (
	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/utils"
)

func (ctl *Controller) ListTasks(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	var req request.ListTaskRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(appErrBadRequest(err))
		return
	}
	data, err := ctl.svc.ListTasks(c.Request.Context(), current, req)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) TaskDetail(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	id, err := utils.MustUint64Param(c, "taskId")
	if err != nil {
		c.Error(err)
		return
	}
	data, err := ctl.svc.GetTask(c.Request.Context(), current, id)
	if err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, data)
}
func (ctl *Controller) RetryTaskItem(c *gin.Context) {
	current, _ := utils.ContextCurrentUser(c)
	taskID, err := utils.MustUint64Param(c, "taskId")
	if err != nil {
		c.Error(err)
		return
	}
	taskItemID, err := utils.MustUint64Param(c, "taskItemId")
	if err != nil {
		c.Error(err)
		return
	}
	if err := ctl.svc.RetryTaskItem(c.Request.Context(), current, taskID, taskItemID); err != nil {
		c.Error(err)
		return
	}
	ctl.ok(c, nil)
}
