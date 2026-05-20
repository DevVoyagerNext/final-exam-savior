package request

import "final-exam-savior/backend/internal/platform"

type RegisterRequest struct {
	Email           string `json:"email"`
	EmailCode       string `json:"emailCode"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
	InviteCode      string `json:"inviteCode"`
	platform.CaptchaPayload
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	platform.CaptchaPayload
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type ChangePasswordRequest struct {
	OldPassword     string `json:"oldPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type ResetPasswordRequest struct {
	Email           string `json:"email"`
	EmailCode       string `json:"emailCode"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type CreateInviteCodeRequest struct {
	CodeType   string `json:"codeType"`
	Code       string `json:"code"`
	TotalQuota uint32 `json:"totalQuota"`
	Remark     string `json:"remark"`
}

type BatchGenerateInviteCodeRequest struct {
	GenerateCount uint32 `json:"generateCount"`
	TotalQuota    uint32 `json:"totalQuota"`
	Remark        string `json:"remark"`
}

type ListInviteCodeRequest struct {
	PageNo   int    `form:"pageNo"`
	PageSize int    `form:"pageSize"`
	Keyword  string `form:"keyword"`
	Status   string `form:"status"`
	BatchNo  string `form:"batchNo"`
}

type CategoryRequest struct {
	Name   string `json:"name"`
	SortNo int    `json:"sortNo"`
}

type UpdateCategoryRequest struct {
	Name   string `json:"name"`
	SortNo int    `json:"sortNo"`
	Status string `json:"status"`
}

type ListFileRequest struct {
	PageNo         int    `form:"pageNo"`
	PageSize       int    `form:"pageSize"`
	Keyword        string `form:"keyword"`
	CategoryID     uint64 `form:"categoryId"`
	Visibility     string `form:"visibility"`
	GenerateStatus string `form:"generateStatus"`
	UploadUserID   uint64 `form:"uploadUserId"`
}

type ListTaskRequest struct {
	PageNo   int    `form:"pageNo"`
	PageSize int    `form:"pageSize"`
	Status   string `form:"status"`
}

type ListNotificationRequest struct {
	PageNo   int    `form:"pageNo"`
	PageSize int    `form:"pageSize"`
	Status   string `form:"status"`
	Type     string `form:"type"`
}

type ListUserRequest struct {
	PageNo   int    `form:"pageNo"`
	PageSize int    `form:"pageSize"`
	Email    string `form:"email"`
	Status   string `form:"status"`
}
