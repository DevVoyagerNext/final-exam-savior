package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"final-exam-savior/backend/internal/dto/response"
	"final-exam-savior/backend/internal/service"
)

type Controller struct {
	svc *service.Service
}

func New(svc *service.Service) *Controller {
	return &Controller{svc: svc}
}
func (ctl *Controller) ok(ctx *gin.Context, data any) {
	response.OK(ctx, data)
}
func (ctl *Controller) abort(ctx *gin.Context, httpStatus int, code int, message string, data any) {
	response.Fail(ctx, httpStatus, code, message, data)
}
func appErrBadRequest(err error) error {
	return &service.AppError{
		HTTPStatus: http.StatusBadRequest,
		Code:       40001,
		Message:    "参数错误",
		Err:        err,
	}
}
