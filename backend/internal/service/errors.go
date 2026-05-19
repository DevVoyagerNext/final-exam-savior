package service

import (
	"errors"
	"fmt"
	"net/http"
)

const (
	codeOK              = 0
	codeBadRequest      = 40001
	codeBusiness        = 40002
	codeUnauthorized    = 40101
	codeForbidden       = 40301
	codeNotFound        = 40401
	codeConflict        = 40901
	codeTooManyRequests = 42901
	codeInternal        = 50001
)

type AppError struct {
	HTTPStatus int
	Code       int
	Message    string
	Err        error
}

func (e *AppError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}
func (e *AppError) Unwrap() error { return e.Err }

func newError(httpStatus, code int, message string, err error) *AppError {
	return &AppError{HTTPStatus: httpStatus, Code: code, Message: message, Err: err}
}
func normalizeErr(err error) error {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return newError(http.StatusInternalServerError, codeInternal, "系统异常", err)
}
