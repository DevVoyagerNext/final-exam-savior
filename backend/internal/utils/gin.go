package utils

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/service"
)

func ContextCurrentUser(c *gin.Context) (*service.CurrentUser, bool) {
	value, ok := c.Get("current_user")
	if !ok {
		return nil, false
	}
	current, ok := value.(*service.CurrentUser)
	return current, ok
}

func MustUint64Param(c *gin.Context, name string) (uint64, error) {
	value := strings.TrimSpace(c.Param(name))
	if value == "" {
		return 0, &service.AppError{HTTPStatus: http.StatusBadRequest, Code: 40001, Message: "路径参数缺失"}
	}
	var id uint64
	if _, err := fmt.Sscanf(value, "%d", &id); err != nil {
		return 0, &service.AppError{HTTPStatus: http.StatusBadRequest, Code: 40001, Message: "路径参数格式错误", Err: err}
	}
	return id, nil
}
