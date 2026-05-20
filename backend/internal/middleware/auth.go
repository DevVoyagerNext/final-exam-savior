package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/dto/response"
	"final-exam-savior/backend/internal/service"
)

func AuthRequired(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		current, ok := authenticateCurrentUser(c, svc)
		if !ok {
			return
		}
		c.Set("current_user", current)
		c.Next()
	}
}

func AdminRequired(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		current, ok := authenticateCurrentUser(c, svc)
		if !ok {
			return
		}
		if current.User.Role != "ADMIN" {
			response.Fail(c, http.StatusForbidden, 40301, "无权限", nil)
			c.Abort()
			return
		}
		c.Set("current_user", current)
		c.Next()
	}
}

func authenticateCurrentUser(c *gin.Context, svc *service.Service) (*service.CurrentUser, bool) {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		response.Fail(c, http.StatusUnauthorized, 40101, "未登录或登录态失效", nil)
		c.Abort()
		return nil, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	current, err := svc.ValidateAccessToken(c.Request.Context(), token)
	if err != nil {
		c.Error(err)
		c.Abort()
		return nil, false
	}
	return current, true
}
