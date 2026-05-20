package router

import (
	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/controller"
)

func RegisterAuthRoutes(r *gin.RouterGroup, c *controller.Controller, authRequired gin.HandlerFunc) {
	auth := r.Group("/auth")
	{
		auth.POST("/register/email-code/send", c.SendRegisterCode)
		auth.POST("/register", c.Register)
		auth.POST("/password-reset/email-code/send", c.SendResetCode)
		auth.POST("/password-reset/confirm", c.ResetPassword)
		auth.POST("/login", c.Login)
		auth.POST("/refresh", c.RefreshToken)
		auth.GET("/me", authRequired, c.Me)
		auth.POST("/logout", authRequired, c.Logout)
		auth.POST("/password/change", authRequired, c.ChangePassword)
	}
}
