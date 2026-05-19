package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"final-exam-savior/backend/internal/dto/request"
	"final-exam-savior/backend/internal/model"
	"final-exam-savior/backend/internal/platform"
)

func (s *Service) ValidateSession(ctx context.Context, tokenString string) (*CurrentUser, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(_ *jwt.Token) (any, error) {
		return []byte(s.cfg.Auth.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, newError(http.StatusUnauthorized, codeUnauthorized, "登录态无效", err)
	}
	jti, _ := claims["jti"].(string)
	sub, _ := claims["sub"].(string)
	if jti == "" || sub == "" {
		return nil, newError(http.StatusUnauthorized, codeUnauthorized, "登录态无效", nil)
	}

	var session model.UserSession
	if err := s.dao.Gorm().WithContext(ctx).
		Where("session_token = ? AND status = ? AND expires_at > ?", jti, "ACTIVE", time.Now()).
		First(&session).Error; err != nil {
		return nil, newError(http.StatusUnauthorized, codeUnauthorized, "登录态已失效", err)
	}
	var user model.User
	if err := s.dao.Gorm().WithContext(ctx).First(&user, session.UserID).Error; err != nil {
		return nil, newError(http.StatusUnauthorized, codeUnauthorized, "用户不存在", err)
	}
	return &CurrentUser{User: user, Session: session}, nil
}

func (s *Service) SendRegisterCode(ctx context.Context, email string, captcha platform.CaptchaPayload) (map[string]any, error) {
	return s.sendEmailCode(ctx, "REGISTER", email, captcha)
}

func (s *Service) SendResetCode(ctx context.Context, email string, captcha platform.CaptchaPayload) error {
	_, err := s.sendEmailCode(ctx, "RESET_PASSWORD", email, captcha)
	return err
}

func (s *Service) sendEmailCode(ctx context.Context, bizType, email string, captcha platform.CaptchaPayload) (map[string]any, error) {
	email = normalizeEmail(email)
	if err := validateQQEmail(email); err != nil {
		return nil, err
	}
	if err := s.captcha.Validate(ctx, captcha); err != nil {
		return nil, newError(http.StatusBadRequest, codeBusiness, "极验验证失败，请重新验证", err)
	}
	if err := s.checkAndIncreaseSendLimit(ctx, email); err != nil {
		return nil, err
	}
	code, err := randomDigits(6)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "生成验证码失败", err)
	}
	record := EmailCodeRecord{
		CodeHash: sha256Hex(code),
		ExpireAt: time.Now().Add(3 * time.Minute),
	}
	data, err := json.Marshal(record)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "序列化验证码失败", err)
	}
	key := s.emailCodeKey(bizType, email)
	if err := s.dao.Redis().Set(ctx, key, data, 3*time.Minute).Err(); err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "保存验证码失败", err)
	}
	subject := "期末救星验证码"
	body := fmt.Sprintf("你的验证码是：%s，3 分钟内有效。", code)
	if err := s.mailer.Send(ctx, email, subject, body); err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "发送验证码失败", err)
	}
	return map[string]any{
		"expireSeconds":        180,
		"nextSendAfterSeconds": 60,
	}, nil
}

func (s *Service) Register(ctx context.Context, req request.RegisterRequest) (map[string]any, error) {
	email := normalizeEmail(req.Email)
	if err := validateQQEmail(email); err != nil {
		return nil, err
	}
	if err := validatePasswordPair(req.Password, req.ConfirmPassword); err != nil {
		return nil, err
	}
	if err := s.captcha.Validate(ctx, req.CaptchaPayload); err != nil {
		return nil, newError(http.StatusBadRequest, codeBusiness, "极验验证失败，请重新验证", err)
	}
	if err := s.consumeEmailCode(ctx, "REGISTER", email, req.EmailCode); err != nil {
		return nil, err
	}

	var result map[string]any
	err := s.dao.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&model.User{}).Where("email = ?", email).Count(&exists).Error; err != nil {
			return fmt.Errorf("check user exists: %w", err)
		}
		if exists > 0 {
			return newError(http.StatusConflict, codeBusiness, "该邮箱已注册", nil)
		}

		var invite model.InviteCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code = ? AND status = ? AND remaining_quota > 0", req.InviteCode, "ACTIVE").
			First(&invite).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newError(http.StatusBadRequest, codeBusiness, "邀请码无效或已失效", err)
			}
			return fmt.Errorf("query invite code: %w", err)
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}

		now := time.Now()
		user := model.User{
			Email:        email,
			PasswordHash: string(hash),
			Role:         "USER",
			Status:       "ENABLED",
			RegisteredAt: now,
		}
		if err := tx.Create(&user).Error; err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		if invite.RemainingQuota <= 1 {
			if err := tx.Delete(&invite).Error; err != nil {
				return fmt.Errorf("delete exhausted invite code: %w", err)
			}
		} else if err := tx.Model(&invite).Update("remaining_quota", invite.RemainingQuota-1).Error; err != nil {
			return fmt.Errorf("decrease invite quota: %w", err)
		}

		tokenData, err := s.createSessionToken(ctx, tx, user, "", "")
		if err != nil {
			return err
		}
		result = tokenData
		return nil
	})
	if err != nil {
		return nil, normalizeErr(err)
	}
	return result, nil
}

func (s *Service) Login(ctx context.Context, req request.LoginRequest, loginIP string, userAgent string) (map[string]any, error) {
	email := normalizeEmail(req.Email)
	if err := s.captcha.Validate(ctx, req.CaptchaPayload); err != nil {
		return nil, newError(http.StatusBadRequest, codeBusiness, "极验验证失败，请重新验证", err)
	}
	if err := s.checkLoginBan(ctx, email); err != nil {
		return nil, err
	}

	var user model.User
	if err := s.dao.Gorm().WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		_ = s.increaseLoginFailure(ctx, email)
		return nil, newError(http.StatusUnauthorized, codeBusiness, "邮箱或密码错误", err)
	}
	if user.Status != "ENABLED" {
		return nil, newError(http.StatusForbidden, codeBusiness, "账号已被禁用", nil)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		_ = s.increaseLoginFailure(ctx, email)
		return nil, newError(http.StatusUnauthorized, codeBusiness, "邮箱或密码错误", err)
	}
	if err := s.clearLoginFailure(ctx, email); err != nil {
		return nil, err
	}

	var result map[string]any
	err := s.dao.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Update("last_login_at", now).Error; err != nil {
			return fmt.Errorf("update last login: %w", err)
		}
		tokenData, err := s.createSessionToken(ctx, tx, user, loginIP, userAgent)
		if err != nil {
			return err
		}
		result = tokenData
		return nil
	})
	if err != nil {
		return nil, normalizeErr(err)
	}
	return result, nil
}

func (s *Service) Logout(ctx context.Context, current *CurrentUser) error {
	now := time.Now()
	reason := "LOGOUT"
	if err := s.dao.Gorm().WithContext(ctx).Model(&model.UserSession{}).
		Where("id = ? AND status = ?", current.Session.ID, "ACTIVE").
		Updates(map[string]any{
			"status":         "INVALIDATED",
			"invalidated_at": now,
			"invalid_reason": reason,
		}).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "退出登录失败", err)
	}
	return nil
}

func (s *Service) Me(_ context.Context, current *CurrentUser) map[string]any {
	return map[string]any{
		"id":           current.User.ID,
		"email":        current.User.Email,
		"role":         current.User.Role,
		"status":       current.User.Status,
		"registeredAt": formatTime(current.User.RegisteredAt),
	}
}

func (s *Service) ChangePassword(ctx context.Context, current *CurrentUser, req request.ChangePasswordRequest) error {
	if err := validatePasswordPair(req.NewPassword, req.ConfirmPassword); err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(current.User.PasswordHash), []byte(req.OldPassword)); err != nil {
		return newError(http.StatusBadRequest, codeBusiness, "旧密码错误", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "密码加密失败", err)
	}
	return normalizeErr(s.invalidateAllSessionsAndUpdatePassword(ctx, current.User.ID, string(hash), "CHANGE_PASSWORD"))
}

func (s *Service) ResetPassword(ctx context.Context, req request.ResetPasswordRequest) error {
	email := normalizeEmail(req.Email)
	if err := validatePasswordPair(req.NewPassword, req.ConfirmPassword); err != nil {
		return err
	}
	if err := s.consumeEmailCode(ctx, "RESET_PASSWORD", email, req.EmailCode); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "密码加密失败", err)
	}

	var user model.User
	if err := s.dao.Gorm().WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return newError(http.StatusNotFound, codeNotFound, "用户不存在", err)
	}
	return normalizeErr(s.invalidateAllSessionsAndUpdatePassword(ctx, user.ID, string(hash), "RESET_PASSWORD"))
}

func (s *Service) invalidateAllSessionsAndUpdatePassword(ctx context.Context, userID uint64, hash string, reason string) error {
	return s.dao.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("password_hash", hash).Error; err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		now := time.Now()
		if err := tx.Model(&model.UserSession{}).
			Where("user_id = ? AND status = ?", userID, "ACTIVE").
			Updates(map[string]any{
				"status":         "INVALIDATED",
				"invalidated_at": now,
				"invalid_reason": reason,
			}).Error; err != nil {
			return fmt.Errorf("invalidate sessions: %w", err)
		}
		return nil
	})
}

func (s *Service) createSessionToken(ctx context.Context, tx *gorm.DB, user model.User, loginIP, userAgent string) (map[string]any, error) {
	sessionID := uuid.NewString()
	expireAt := time.Now().Add(s.cfg.Auth.TokenTTL)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  fmt.Sprintf("%d", user.ID),
		"jti":  sessionID,
		"role": user.Role,
		"exp":  expireAt.Unix(),
		"iss":  s.cfg.Auth.Issuer,
	})
	signed, err := token.SignedString([]byte(s.cfg.Auth.JWTSecret))
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "签发登录态失败", err)
	}
	session := model.UserSession{
		UserID:       user.ID,
		SessionToken: sessionID,
		Status:       "ACTIVE",
		IssuedAt:     time.Now(),
		ExpiresAt:    expireAt,
		LoginIP:      optionalString(loginIP),
		UserAgent:    optionalString(userAgent),
	}
	if err := tx.WithContext(ctx).Create(&session).Error; err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return map[string]any{
		"token":    signed,
		"expireAt": formatTime(expireAt),
		"user": map[string]any{
			"id":     user.ID,
			"email":  user.Email,
			"role":   user.Role,
			"status": user.Status,
		},
	}, nil
}

func (s *Service) checkAndIncreaseSendLimit(ctx context.Context, email string) error {
	windows := []struct {
		TTL   time.Duration
		Limit int
	}{
		{TTL: time.Minute, Limit: 1},
		{TTL: 5 * time.Minute, Limit: 3},
		{TTL: 3 * time.Hour, Limit: 5},
		{TTL: 24 * time.Hour, Limit: 7},
	}
	pipe := s.dao.Redis().TxPipeline()
	cmds := make([]*redis.IntCmd, 0, len(windows))
	keys := make([]string, 0, len(windows))
	for _, window := range windows {
		key := fmt.Sprintf("email:send_limit:%s:%d", email, int(window.TTL.Seconds()))
		keys = append(keys, key)
		cmds = append(cmds, pipe.Incr(ctx, key))
		pipe.Expire(ctx, key, window.TTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "验证码频控检查失败", err)
	}
	for i, cmd := range cmds {
		if int(cmd.Val()) > windows[i].Limit {
			return newError(http.StatusTooManyRequests, codeTooManyRequests, "验证码发送过于频繁，请稍后再试", nil)
		}
		_ = keys
	}
	return nil
}

func (s *Service) emailCodeKey(bizType string, email string) string {
	return fmt.Sprintf("email:code:%s:%s", bizType, email)
}

func (s *Service) consumeEmailCode(ctx context.Context, bizType, email, code string) error {
	key := s.emailCodeKey(bizType, email)
	data, err := s.dao.Redis().Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return newError(http.StatusBadRequest, codeBusiness, "验证码错误、过期或已失效", err)
		}
		return newError(http.StatusInternalServerError, codeInternal, "读取验证码失败", err)
	}
	var record EmailCodeRecord
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "验证码状态损坏", err)
	}
	if time.Now().After(record.ExpireAt) {
		_ = s.dao.Redis().Del(ctx, key).Err()
		return newError(http.StatusBadRequest, codeBusiness, "验证码已过期", nil)
	}
	if record.CodeHash != sha256Hex(code) {
		record.AttemptCnt++
		if record.AttemptCnt >= 3 {
			_ = s.dao.Redis().Del(ctx, key).Err()
			return newError(http.StatusBadRequest, codeBusiness, "验证码错误次数过多，请重新获取", nil)
		}
		updated, _ := json.Marshal(record)
		_ = s.dao.Redis().Set(ctx, key, updated, time.Until(record.ExpireAt)).Err()
		return newError(http.StatusBadRequest, codeBusiness, "验证码错误", nil)
	}
	if err := s.dao.Redis().Del(ctx, key).Err(); err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "消费验证码失败", err)
	}
	return nil
}

func (s *Service) checkLoginBan(ctx context.Context, email string) error {
	banKey := fmt.Sprintf("login:ban:%s", email)
	ttl, err := s.dao.Redis().TTL(ctx, banKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return newError(http.StatusInternalServerError, codeInternal, "检查登录风控失败", err)
	}
	if ttl > 0 {
		return newError(http.StatusTooManyRequests, codeTooManyRequests, fmt.Sprintf("登录受限，请 %d 秒后再试", int(ttl.Seconds())), nil)
	}
	return nil
}

func (s *Service) increaseLoginFailure(ctx context.Context, email string) error {
	windows := []struct {
		TTL    time.Duration
		Limit  int
		BanFor time.Duration
	}{
		{TTL: time.Hour, Limit: 3, BanFor: time.Hour},
		{TTL: 3 * time.Hour, Limit: 5, BanFor: 3 * time.Hour},
		{TTL: 24 * time.Hour, Limit: 7, BanFor: 24 * time.Hour},
	}
	var maxBan time.Duration
	pipe := s.dao.Redis().TxPipeline()
	cmds := make([]*redis.IntCmd, 0, len(windows))
	for _, window := range windows {
		key := fmt.Sprintf("login:fail:%s:%d", email, int(window.TTL.Seconds()))
		cmds = append(cmds, pipe.Incr(ctx, key))
		pipe.Expire(ctx, key, window.TTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "记录登录失败次数失败", err)
	}
	for i, cmd := range cmds {
		if int(cmd.Val()) >= windows[i].Limit && windows[i].BanFor > maxBan {
			maxBan = windows[i].BanFor
		}
	}
	if maxBan > 0 {
		if err := s.dao.Redis().Set(ctx, fmt.Sprintf("login:ban:%s", email), "1", maxBan).Err(); err != nil {
			return newError(http.StatusInternalServerError, codeInternal, "写入登录封禁失败", err)
		}
	}
	return nil
}

func (s *Service) clearLoginFailure(ctx context.Context, email string) error {
	keys := []string{
		fmt.Sprintf("login:ban:%s", email),
		fmt.Sprintf("login:fail:%s:%d", email, int(time.Hour.Seconds())),
		fmt.Sprintf("login:fail:%s:%d", email, int((3 * time.Hour).Seconds())),
		fmt.Sprintf("login:fail:%s:%d", email, int((24 * time.Hour).Seconds())),
	}
	if err := s.dao.Redis().Del(ctx, keys...).Err(); err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "清理登录失败次数失败", err)
	}
	return nil
}
