package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/dto/response"
	"final-exam-savior/backend/internal/service"
)

func Error() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		err := c.Errors.Last().Err
		var appErr *service.AppError
		if errors.As(err, &appErr) {
			response.Fail(c, appErr.HTTPStatus, appErr.Code, appErr.Message, nil)
			return
		}
		response.Fail(c, http.StatusInternalServerError, 50001, "系统异常", nil)
	}
}
