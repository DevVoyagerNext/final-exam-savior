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
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			response.Fail(c, http.StatusUnauthorized, 40101, "未登录或登录态失效", nil)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		current, err := svc.ValidateSession(c.Request.Context(), token)
		if err != nil {
			c.Error(err)
			c.Abort()
			return
		}
		c.Set("current_user", current)
		c.Next()
	}
}
