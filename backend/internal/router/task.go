package router

import (
	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/controller"
)

func RegisterTaskRoutes(r *gin.RouterGroup, c *controller.Controller, authRequired gin.HandlerFunc) {
	r.GET("/tasks", authRequired, c.ListTasks)
	r.GET("/tasks/:taskId", authRequired, c.TaskDetail)
	r.POST("/admin/tasks/:taskId/items/:taskItemId/retry", authRequired, c.RetryTaskItem)
}
