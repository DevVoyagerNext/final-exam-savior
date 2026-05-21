package service

import (
	"github.com/golang-jwt/jwt/v5"

	"time"

	"final-exam-savior/backend/internal/model"
)

type CurrentUser struct {
	User      model.User
	TokenID   string
	TokenType string
}
type EmailCodeRecord struct {
	CodeHash   string    `json:"codeHash"`
	ExpireAt   time.Time `json:"expireAt"`
	AttemptCnt int       `json:"attemptCnt"`
}
type AuthClaims struct {
	Email      string `json:"email"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	TokenType  string `json:"tokenType"`
	TokenID    string `json:"tokenId"`
	jwt.RegisteredClaims
}
type GenerateEvent struct {
	TaskID uint64 `json:"taskId"`
}
