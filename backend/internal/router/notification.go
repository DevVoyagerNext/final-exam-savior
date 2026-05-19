package router

import (
	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/controller"
)

func RegisterNotificationRoutes(r *gin.RouterGroup, c *controller.Controller, authRequired gin.HandlerFunc) {
	r.GET("/notifications", authRequired, c.ListNotifications)
	r.GET("/notifications/unread-count", authRequired, c.UnreadCount)
	r.GET("/notifications/:notificationId", authRequired, c.NotificationDetail)
	r.POST("/notifications/:notificationId/read", authRequired, c.MarkNotificationRead)
	r.POST("/notifications/read/batch", authRequired, c.MarkNotificationReadBatch)
}
