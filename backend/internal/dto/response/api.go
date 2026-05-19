package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID any    `json:"requestId"`
}

func OK(c *gin.Context, data any) {
	requestID, _ := c.Get("request_id")
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: data, RequestID: requestID})
}

func Fail(c *gin.Context, httpStatus int, code int, message string, data any) {
	requestID, _ := c.Get("request_id")
	c.AbortWithStatusJSON(httpStatus, APIResponse{Code: code, Message: message, Data: data, RequestID: requestID})
}
