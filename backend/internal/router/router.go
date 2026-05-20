package router

import (
	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/controller"
	"final-exam-savior/backend/internal/middleware"
	"final-exam-savior/backend/internal/service"
)

func New(c *controller.Controller, svc *service.Service) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.AccessLog())
	r.Use(middleware.Error())

	r.GET("/storage/local/*objectKey", c.ServeLocalStorage)

	v1 := r.Group("/api/v1")
	authRequired := middleware.AuthRequired(svc)
	adminRequired := middleware.AdminRequired(svc)

	RegisterAuthRoutes(v1, c, authRequired)
	RegisterFileRoutes(v1, c, authRequired)
	RegisterTaskRoutes(v1, c, authRequired)
	RegisterNotificationRoutes(v1, c, authRequired)
	RegisterAdminRoutes(v1, c, adminRequired)

	return r
}
