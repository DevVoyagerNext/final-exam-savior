package service

import (
	"time"

	"final-exam-savior/backend/internal/model"
)

type CurrentUser struct {
	User    model.User
	Session model.UserSession
}
type EmailCodeRecord struct {
	CodeHash   string    `json:"codeHash"`
	ExpireAt   time.Time `json:"expireAt"`
	AttemptCnt int       `json:"attemptCnt"`
}
type GenerateEvent struct {
	TaskID uint64 `json:"taskId"`
}
type PreviewEvent struct {
	ConversionTaskID uint64 `json:"conversionTaskId"`
}
