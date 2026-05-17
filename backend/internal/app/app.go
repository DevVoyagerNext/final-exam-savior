package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"final-exam-savior/backend/internal/config"
	"final-exam-savior/backend/internal/model"
	"final-exam-savior/backend/internal/platform"
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

type App struct {
	cfg          config.Config
	db           *gorm.DB
	redis        *redis.Client
	mailer       platform.Mailer
	captcha      platform.CaptchaValidator
	storage      platform.ObjectStorage
	localStorage *platform.LocalFileStorage
	ai           platform.AIClient
	parser       platform.Parser
	converter    platform.Converter
	httpClient   *http.Client
}

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

func New(ctx context.Context, cfg config.Config) (*App, error) {
	db, err := openDB(cfg)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	storage, localStorage, err := buildStorage(cfg)
	if err != nil {
		return nil, err
	}

	app := &App{
		cfg:          cfg,
		db:           db,
		redis:        rdb,
		mailer:       platform.NewSMTPMailer(cfg.SMTP),
		captcha:      platform.NewGeetestValidator(cfg.Geetest),
		storage:      storage,
		localStorage: localStorage,
		ai:           platform.NewOpenAICompatClient(cfg.AI),
		parser:       platform.NewHTTPParser(cfg.Parser),
		converter:    platform.NewHTTPConverter(cfg.Preview),
		httpClient:   &http.Client{Timeout: 2 * time.Minute},
	}

	if cfg.Database.AutoMigrate {
		if err := db.AutoMigrate(model.AutoMigrateModels()...); err != nil {
			return nil, fmt.Errorf("auto migrate: %w", err)
		}
	}

	if err := app.ensureSeeds(ctx); err != nil {
		return nil, err
	}
	if err := app.ensureRedisGroups(ctx); err != nil {
		return nil, err
	}
	return app, nil
}

func openDB(cfg config.Config) (*gorm.DB, error) {
	gormCfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)}
	db, err := gorm.Open(mysql.Open(cfg.Database.DSN), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Minute)
	return db, nil
}

func buildStorage(cfg config.Config) (platform.ObjectStorage, *platform.LocalFileStorage, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Storage.Mode)) {
	case "", "local":
		localStorage, err := platform.NewLocalFileStorage(cfg.Storage, cfg.Auth.JWTSecret)
		if err != nil {
			return nil, nil, fmt.Errorf("init local storage: %w", err)
		}
		return localStorage, localStorage, nil
	case "oss":
		storage, err := platform.NewOSSStorage(cfg.OSS)
		if err != nil {
			return nil, nil, err
		}
		return storage, nil, nil
	default:
		return nil, nil, fmt.Errorf("unsupported storage mode: %s", cfg.Storage.Mode)
	}
}

func (a *App) DB() *gorm.DB { return a.db }

func (a *App) ensureSeeds(ctx context.Context) error {
	var count int64
	if err := a.db.WithContext(ctx).Model(&model.FileCategory{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count categories: %w", err)
	}
	if count == 0 {
		defaultCategory := model.FileCategory{
			Name:      a.cfg.App.DefaultCategoryName,
			IsBuiltin: true,
			Status:    "ENABLED",
			SortNo:    0,
		}
		if err := a.db.WithContext(ctx).Create(&defaultCategory).Error; err != nil {
			return fmt.Errorf("create default category: %w", err)
		}
	}

	var admin model.User
	err := a.db.WithContext(ctx).Where("email = ?", a.cfg.App.DefaultAdminEmail).First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) && a.cfg.App.DefaultAdminEmail != "" && a.cfg.App.DefaultAdminPass != "" {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(a.cfg.App.DefaultAdminPass), bcrypt.DefaultCost)
		if hashErr != nil {
			return fmt.Errorf("hash default admin password: %w", hashErr)
		}
		now := time.Now()
		admin = model.User{
			Email:        strings.ToLower(strings.TrimSpace(a.cfg.App.DefaultAdminEmail)),
			PasswordHash: string(hash),
			Role:         "ADMIN",
			Status:       "ENABLED",
			RegisteredAt: now,
		}
		if createErr := a.db.WithContext(ctx).Create(&admin).Error; createErr != nil {
			return fmt.Errorf("create default admin: %w", createErr)
		}
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query default admin: %w", err)
	}
	return nil
}

func (a *App) ensureRedisGroups(ctx context.Context) error {
	for _, stream := range []string{a.cfg.Redis.GenerateStream, a.cfg.Redis.PreviewStream} {
		err := a.redis.XGroupCreateMkStream(ctx, stream, a.cfg.Redis.ConsumerGroup, "$").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			return fmt.Errorf("create redis group %s: %w", stream, err)
		}
	}
	return nil
}

func (a *App) Close() error {
	sqlDB, err := a.db.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}
	return a.redis.Close()
}

func (a *App) ResolveLocalObjectPath(objectKey string, expireAt string, signature string) (string, error) {
	if a.localStorage == nil {
		return "", newError(http.StatusNotFound, codeNotFound, "本地存储未启用", nil)
	}
	expireAtUnix, err := strconv.ParseInt(strings.TrimSpace(expireAt), 10, 64)
	if err != nil {
		return "", newError(http.StatusBadRequest, codeBadRequest, "本地文件访问参数错误", err)
	}
	fullPath, err := a.localStorage.VerifyAndResolve(strings.TrimPrefix(objectKey, "/"), expireAtUnix, signature)
	if err != nil {
		return "", newError(http.StatusForbidden, codeForbidden, "本地文件访问无效或已过期", err)
	}
	return fullPath, nil
}

func (a *App) StartWorkers(ctx context.Context) {
	go a.consumeGenerateStream(ctx)
	go a.consumePreviewStream(ctx)
}

func (a *App) consumeGenerateStream(ctx context.Context) {
	for ctx.Err() == nil {
		streams, err := a.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    a.cfg.Redis.ConsumerGroup,
			Consumer: a.cfg.Redis.ConsumerName,
			Streams:  []string{a.cfg.Redis.GenerateStream, ">"},
			Count:    1,
			Block:    a.cfg.Redis.BlockTimeout,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || strings.Contains(err.Error(), "context canceled") {
				continue
			}
			time.Sleep(time.Second)
			continue
		}
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				if err := a.handleGenerateMessage(ctx, msg); err == nil {
					_ = a.redis.XAck(ctx, a.cfg.Redis.GenerateStream, a.cfg.Redis.ConsumerGroup, msg.ID).Err()
				}
			}
		}
	}
}

func (a *App) consumePreviewStream(ctx context.Context) {
	for ctx.Err() == nil {
		streams, err := a.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    a.cfg.Redis.ConsumerGroup,
			Consumer: a.cfg.Redis.ConsumerName,
			Streams:  []string{a.cfg.Redis.PreviewStream, ">"},
			Count:    1,
			Block:    a.cfg.Redis.BlockTimeout,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || strings.Contains(err.Error(), "context canceled") {
				continue
			}
			time.Sleep(time.Second)
			continue
		}
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				if err := a.handlePreviewMessage(ctx, msg); err == nil {
					_ = a.redis.XAck(ctx, a.cfg.Redis.PreviewStream, a.cfg.Redis.ConsumerGroup, msg.ID).Err()
				}
			}
		}
	}
}

func (a *App) handleGenerateMessage(ctx context.Context, msg redis.XMessage) error {
	raw, ok := msg.Values["payload"].(string)
	if !ok {
		return fmt.Errorf("generate payload missing")
	}
	var event GenerateEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return fmt.Errorf("unmarshal generate event: %w", err)
	}
	return a.processGenerateTask(ctx, event.TaskID)
}

func (a *App) handlePreviewMessage(ctx context.Context, msg redis.XMessage) error {
	raw, ok := msg.Values["payload"].(string)
	if !ok {
		return fmt.Errorf("preview payload missing")
	}
	var event PreviewEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return fmt.Errorf("unmarshal preview event: %w", err)
	}
	return a.processPreviewTask(ctx, event.ConversionTaskID)
}

func (a *App) ValidateSession(ctx context.Context, tokenString string) (*CurrentUser, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(_ *jwt.Token) (any, error) {
		return []byte(a.cfg.Auth.JWTSecret), nil
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
	if err := a.db.WithContext(ctx).
		Where("session_token = ? AND status = ? AND expires_at > ?", jti, "ACTIVE", time.Now()).
		First(&session).Error; err != nil {
		return nil, newError(http.StatusUnauthorized, codeUnauthorized, "登录态已失效", err)
	}
	var user model.User
	if err := a.db.WithContext(ctx).First(&user, session.UserID).Error; err != nil {
		return nil, newError(http.StatusUnauthorized, codeUnauthorized, "用户不存在", err)
	}
	return &CurrentUser{User: user, Session: session}, nil
}

func (a *App) SendRegisterCode(ctx context.Context, email string, captcha platform.CaptchaPayload) (map[string]any, error) {
	return a.sendEmailCode(ctx, "REGISTER", email, captcha)
}

func (a *App) SendResetCode(ctx context.Context, email string, captcha platform.CaptchaPayload) error {
	_, err := a.sendEmailCode(ctx, "RESET_PASSWORD", email, captcha)
	return err
}

func (a *App) sendEmailCode(ctx context.Context, bizType, email string, captcha platform.CaptchaPayload) (map[string]any, error) {
	email = normalizeEmail(email)
	if err := validateQQEmail(email); err != nil {
		return nil, err
	}
	if err := a.captcha.Validate(ctx, captcha); err != nil {
		return nil, newError(http.StatusBadRequest, codeBusiness, "极验验证失败，请重新验证", err)
	}
	if err := a.checkAndIncreaseSendLimit(ctx, email); err != nil {
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
	key := a.emailCodeKey(bizType, email)
	if err := a.redis.Set(ctx, key, data, 3*time.Minute).Err(); err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "保存验证码失败", err)
	}
	subject := "期末救星验证码"
	body := fmt.Sprintf("你的验证码是：%s，3 分钟内有效。", code)
	if err := a.mailer.Send(ctx, email, subject, body); err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "发送验证码失败", err)
	}
	return map[string]any{
		"expireSeconds":        180,
		"nextSendAfterSeconds": 60,
	}, nil
}

func (a *App) Register(ctx context.Context, req RegisterRequest) (map[string]any, error) {
	email := normalizeEmail(req.Email)
	if err := validateQQEmail(email); err != nil {
		return nil, err
	}
	if err := validatePasswordPair(req.Password, req.ConfirmPassword); err != nil {
		return nil, err
	}
	if err := a.captcha.Validate(ctx, req.CaptchaPayload); err != nil {
		return nil, newError(http.StatusBadRequest, codeBusiness, "极验验证失败，请重新验证", err)
	}
	if err := a.consumeEmailCode(ctx, "REGISTER", email, req.EmailCode); err != nil {
		return nil, err
	}

	var result map[string]any
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

		tokenData, err := a.createSessionToken(ctx, tx, user, "", "")
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

func (a *App) Login(ctx context.Context, req LoginRequest, loginIP string, userAgent string) (map[string]any, error) {
	email := normalizeEmail(req.Email)
	if err := a.captcha.Validate(ctx, req.CaptchaPayload); err != nil {
		return nil, newError(http.StatusBadRequest, codeBusiness, "极验验证失败，请重新验证", err)
	}
	if err := a.checkLoginBan(ctx, email); err != nil {
		return nil, err
	}

	var user model.User
	if err := a.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		_ = a.increaseLoginFailure(ctx, email)
		return nil, newError(http.StatusUnauthorized, codeBusiness, "邮箱或密码错误", err)
	}
	if user.Status != "ENABLED" {
		return nil, newError(http.StatusForbidden, codeBusiness, "账号已被禁用", nil)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		_ = a.increaseLoginFailure(ctx, email)
		return nil, newError(http.StatusUnauthorized, codeBusiness, "邮箱或密码错误", err)
	}
	if err := a.clearLoginFailure(ctx, email); err != nil {
		return nil, err
	}

	var result map[string]any
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Update("last_login_at", now).Error; err != nil {
			return fmt.Errorf("update last login: %w", err)
		}
		tokenData, err := a.createSessionToken(ctx, tx, user, loginIP, userAgent)
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

func (a *App) Logout(ctx context.Context, current *CurrentUser) error {
	now := time.Now()
	reason := "LOGOUT"
	if err := a.db.WithContext(ctx).Model(&model.UserSession{}).
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

func (a *App) Me(_ context.Context, current *CurrentUser) map[string]any {
	return map[string]any{
		"id":           current.User.ID,
		"email":        current.User.Email,
		"role":         current.User.Role,
		"status":       current.User.Status,
		"registeredAt": formatTime(current.User.RegisteredAt),
	}
}

func (a *App) ChangePassword(ctx context.Context, current *CurrentUser, req ChangePasswordRequest) error {
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
	return normalizeErr(a.invalidateAllSessionsAndUpdatePassword(ctx, current.User.ID, string(hash), "CHANGE_PASSWORD"))
}

func (a *App) ResetPassword(ctx context.Context, req ResetPasswordRequest) error {
	email := normalizeEmail(req.Email)
	if err := validatePasswordPair(req.NewPassword, req.ConfirmPassword); err != nil {
		return err
	}
	if err := a.consumeEmailCode(ctx, "RESET_PASSWORD", email, req.EmailCode); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "密码加密失败", err)
	}

	var user model.User
	if err := a.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return newError(http.StatusNotFound, codeNotFound, "用户不存在", err)
	}
	return normalizeErr(a.invalidateAllSessionsAndUpdatePassword(ctx, user.ID, string(hash), "RESET_PASSWORD"))
}

func (a *App) CreateInviteCode(ctx context.Context, current *CurrentUser, req CreateInviteCodeRequest) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	req.CodeType = strings.ToUpper(strings.TrimSpace(req.CodeType))
	if req.TotalQuota == 0 {
		return newError(http.StatusBadRequest, codeBadRequest, "totalQuota 必须大于 0", nil)
	}
	codeValue := strings.TrimSpace(req.Code)
	if req.CodeType == "RANDOM" {
		randomCode, err := randomAlphaNum(10)
		if err != nil {
			return newError(http.StatusInternalServerError, codeInternal, "生成邀请码失败", err)
		}
		codeValue = randomCode
	}
	if codeValue == "" {
		return newError(http.StatusBadRequest, codeBadRequest, "邀请码不能为空", nil)
	}
	record := model.InviteCode{
		Code:           codeValue,
		CodeType:       req.CodeType,
		TotalQuota:     req.TotalQuota,
		RemainingQuota: req.TotalQuota,
		Status:         "ACTIVE",
		Remark:         optionalString(req.Remark),
		CreatedBy:      current.User.ID,
	}
	if err := a.db.WithContext(ctx).Create(&record).Error; err != nil {
		return newError(http.StatusConflict, codeBusiness, "邀请码创建失败，可能已重复", err)
	}
	return nil
}

func (a *App) BatchGenerateInviteCodes(ctx context.Context, current *CurrentUser, req BatchGenerateInviteCodeRequest) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if req.GenerateCount == 0 || req.GenerateCount > 100 {
		return nil, newError(http.StatusBadRequest, codeBadRequest, "generateCount 必须在 1-100 之间", nil)
	}
	batchNo := fmt.Sprintf("INV-%s-%04d", time.Now().Format("20060102"), time.Now().Nanosecond()%10000)
	codes := make([]map[string]any, 0, req.GenerateCount)
	for i := uint32(0); i < req.GenerateCount; i++ {
		codeValue, err := randomAlphaNum(10)
		if err != nil {
			return nil, newError(http.StatusInternalServerError, codeInternal, "生成邀请码失败", err)
		}
		record := model.InviteCode{
			Code:           codeValue,
			CodeType:       "RANDOM",
			BatchNo:        optionalString(batchNo),
			TotalQuota:     req.TotalQuota,
			RemainingQuota: req.TotalQuota,
			Status:         "ACTIVE",
			Remark:         optionalString(req.Remark),
			CreatedBy:      current.User.ID,
		}
		if err := a.db.WithContext(ctx).Create(&record).Error; err != nil {
			return nil, newError(http.StatusConflict, codeBusiness, "批量生成邀请码失败", err)
		}
		codes = append(codes, inviteCodeDTO(record))
	}
	return map[string]any{
		"batchNo":       batchNo,
		"generateCount": req.GenerateCount,
		"codes":         codes,
	}, nil
}

func (a *App) ListInviteCodes(ctx context.Context, current *CurrentUser, req ListInviteCodeRequest) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	var list []model.InviteCode
	tx := a.db.WithContext(ctx).Model(&model.InviteCode{})
	if req.Keyword != "" {
		tx = tx.Where("code LIKE ?", "%"+req.Keyword+"%")
	}
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	if req.BatchNo != "" {
		tx = tx.Where("batch_no = ?", req.BatchNo)
	}
	return pageQuery(ctx, tx, req.PageNo, req.PageSize, "id DESC", &list, func(item model.InviteCode) map[string]any {
		return inviteCodeDTO(item)
	})
}

func (a *App) UpdateInviteCodeRemark(ctx context.Context, current *CurrentUser, id uint64, remark string) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if err := a.db.WithContext(ctx).Model(&model.InviteCode{}).Where("id = ?", id).Update("remark", optionalString(remark)).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "修改邀请码备注失败", err)
	}
	return nil
}

func (a *App) DeleteInviteCode(ctx context.Context, current *CurrentUser, id uint64) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if err := a.db.WithContext(ctx).Delete(&model.InviteCode{}, id).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "删除邀请码失败", err)
	}
	return nil
}

func (a *App) ListCategories(ctx context.Context) ([]map[string]any, error) {
	var list []model.FileCategory
	if err := a.db.WithContext(ctx).Order("sort_no ASC, id ASC").Find(&list).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询分类失败", err)
	}
	result := make([]map[string]any, 0, len(list))
	for _, item := range list {
		result = append(result, map[string]any{
			"id":        item.ID,
			"name":      item.Name,
			"sortNo":    item.SortNo,
			"status":    item.Status,
			"isDefault": item.IsBuiltin,
		})
	}
	return result, nil
}

func (a *App) CreateCategory(ctx context.Context, current *CurrentUser, req CategoryRequest) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	record := model.FileCategory{
		Name:      strings.TrimSpace(req.Name),
		Status:    "ENABLED",
		SortNo:    req.SortNo,
		IsBuiltin: false,
		CreatedBy: &current.User.ID,
	}
	if record.Name == "" {
		return newError(http.StatusBadRequest, codeBadRequest, "分类名称不能为空", nil)
	}
	if err := a.db.WithContext(ctx).Create(&record).Error; err != nil {
		return newError(http.StatusConflict, codeBusiness, "分类创建失败，可能名称重复", err)
	}
	return nil
}

func (a *App) UpdateCategory(ctx context.Context, current *CurrentUser, categoryID uint64, req UpdateCategoryRequest) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	var record model.FileCategory
	if err := a.db.WithContext(ctx).First(&record, categoryID).Error; err != nil {
		return newError(http.StatusNotFound, codeNotFound, "分类不存在", err)
	}
	if record.IsBuiltin && req.Name != record.Name {
		return newError(http.StatusBadRequest, codeBusiness, "默认分类不可修改名称", nil)
	}
	if record.IsBuiltin && req.Status != "" && req.Status != record.Status {
		return newError(http.StatusBadRequest, codeBusiness, "默认分类不可禁用", nil)
	}
	updates := map[string]any{
		"sort_no": req.SortNo,
	}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if err := a.db.WithContext(ctx).Model(&record).Updates(updates).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "更新分类失败", err)
	}
	return nil
}

func (a *App) DeleteCategory(ctx context.Context, current *CurrentUser, categoryID uint64) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	var record model.FileCategory
	if err := a.db.WithContext(ctx).First(&record, categoryID).Error; err != nil {
		return newError(http.StatusNotFound, codeNotFound, "分类不存在", err)
	}
	if record.IsBuiltin {
		return newError(http.StatusBadRequest, codeBusiness, "默认分类不可删除", nil)
	}
	var count int64
	if err := a.db.WithContext(ctx).Model(&model.LearningFile{}).Where("category_id = ?", categoryID).Count(&count).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "查询分类文件失败", err)
	}
	if count > 0 {
		return newError(http.StatusBadRequest, codeBusiness, "分类下仍存在文件，无法删除", nil)
	}
	if err := a.db.WithContext(ctx).Delete(&record).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "删除分类失败", err)
	}
	return nil
}

func (a *App) ListFiles(ctx context.Context, current *CurrentUser, req ListFileRequest, adminMode bool) (map[string]any, error) {
	tx := a.db.WithContext(ctx).Model(&model.LearningFile{})
	if !adminMode {
		tx = tx.Where("visibility = ? OR upload_user_id = ?", "PUBLIC", current.User.ID)
	}
	if req.Keyword != "" {
		tx = tx.Where("source_file_name LIKE ?", "%"+req.Keyword+"%")
	}
	if req.CategoryID > 0 {
		tx = tx.Where("category_id = ?", req.CategoryID)
	}
	if req.Visibility != "" {
		tx = tx.Where("visibility = ?", req.Visibility)
	}
	if adminMode && req.UploadUserID > 0 {
		tx = tx.Where("upload_user_id = ?", req.UploadUserID)
	}

	var files []model.LearningFile
	page, err := pageQuery(ctx, tx, req.PageNo, req.PageSize, "upload_time DESC", &files, func(file model.LearningFile) map[string]any {
		return map[string]any{}
	})
	if err != nil {
		return nil, err
	}
	list := make([]map[string]any, 0, len(files))
	for _, file := range files {
		dto, dtoErr := a.fileListItem(ctx, file)
		if dtoErr != nil {
			return nil, dtoErr
		}
		if req.GenerateStatus != "" && dto["generateTotalStatus"] != req.GenerateStatus {
			continue
		}
		list = append(list, dto)
	}
	page["list"] = list
	page["total"] = len(list)
	return page, nil
}

func (a *App) GetFileDetail(ctx context.Context, current *CurrentUser, fileID uint64) (map[string]any, error) {
	file, err := a.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, err
	}
	item, err := a.fileListItem(ctx, file)
	if err != nil {
		return nil, err
	}
	var preview model.FilePreviewRecord
	_ = a.db.WithContext(ctx).Where("file_id = ?", fileID).First(&preview).Error
	var record model.FileGenerateRecord
	_ = a.db.WithContext(ctx).Where("file_id = ?", fileID).First(&record).Error
	var items []model.FileGenerateRecordItem
	_ = a.db.WithContext(ctx).Where("generate_record_id = ?", record.ID).Find(&items).Error

	generateItems := make([]map[string]any, 0, len(items))
	for _, entry := range items {
		generateItems = append(generateItems, map[string]any{
			"itemType":        entry.ItemType,
			"itemStatus":      entry.ItemStatus,
			"resultObjectUrl": entry.ResultObjectURL,
		})
	}

	item["sourceFileHash"] = file.SourceFileHash
	item["sourceFileUrl"] = file.SourceObjectURL
	item["generateRecord"] = map[string]any{
		"totalStatus":     record.TotalStatus,
		"lastGeneratedAt": formatTimePtr(record.LastGeneratedAt),
		"items":           generateItems,
	}
	item["previewRecord"] = map[string]any{
		"previewMode":      preview.PreviewMode,
		"previewStatus":    preview.PreviewStatus,
		"previewObjectUrl": preview.PreviewObjectURL,
	}
	return item, nil
}

func (a *App) UploadFile(ctx context.Context, current *CurrentUser, fileHeader *multipart.FileHeader, categoryID uint64, visibility string) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if fileHeader == nil {
		return nil, newError(http.StatusBadRequest, codeBadRequest, "文件不能为空", nil)
	}
	if categoryID == 0 {
		return nil, newError(http.StatusBadRequest, codeBadRequest, "categoryId 不能为空", nil)
	}
	var category model.FileCategory
	if err := a.db.WithContext(ctx).First(&category, categoryID).Error; err != nil {
		return nil, newError(http.StatusBadRequest, codeBusiness, "分类不存在", err)
	}
	data, err := platform.ReadMultipartFile(fileHeader)
	if err != nil {
		return nil, newError(http.StatusBadRequest, codeBadRequest, "读取上传文件失败", err)
	}
	hash := sha256HexBytes(data)
	objectKey := platform.BuildObjectKey("source", fileHeader.Filename)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	sourceURL, err := a.storage.Upload(ctx, objectKey, contentType, data)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "上传源文件到 OSS 失败", err)
	}

	var response map[string]any
	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		var file model.LearningFile
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_file_hash = ?", hash).First(&file).Error
		reuse := findErr == nil
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			file = model.LearningFile{
				SourceFileHash:  hash,
				SourceFileName:  fileHeader.Filename,
				SourceFileType:  contentType,
				SourceFileSize:  uint64(len(data)),
				SourceObjectURL: sourceURL,
				CategoryID:      categoryID,
				Visibility:      visibility,
				UploadUserID:    current.User.ID,
				UploadTime:      now,
			}
			if err := tx.Create(&file).Error; err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			previewMode := detectPreviewMode(contentType)
			previewStatus := "SUCCESS"
			if previewMode == "CONVERT_TO_PDF" {
				previewStatus = "PENDING"
			}
			preview := model.FilePreviewRecord{
				FileID:        file.ID,
				PreviewMode:   previewMode,
				PreviewStatus: previewStatus,
			}
			if err := tx.Create(&preview).Error; err != nil {
				return fmt.Errorf("create preview record: %w", err)
			}
			generateRecord := model.FileGenerateRecord{
				FileID:      file.ID,
				TotalStatus: "PENDING",
			}
			if err := tx.Create(&generateRecord).Error; err != nil {
				return fmt.Errorf("create generate record: %w", err)
			}
			for _, itemType := range []string{"QUESTION", "KNOWLEDGE", "EXTENDED"} {
				item := model.FileGenerateRecordItem{
					GenerateRecordID: generateRecord.ID,
					ItemType:         itemType,
					ItemStatus:       "PENDING",
				}
				if err := tx.Create(&item).Error; err != nil {
					return fmt.Errorf("create generate record item: %w", err)
				}
			}
		} else if findErr != nil {
			return fmt.Errorf("query file by hash: %w", findErr)
		} else {
			if err := tx.Model(&file).Updates(map[string]any{
				"source_file_name":  fileHeader.Filename,
				"source_file_type":  contentType,
				"source_file_size":  uint64(len(data)),
				"source_object_url": sourceURL,
				"category_id":       categoryID,
				"visibility":        visibility,
				"upload_user_id":    current.User.ID,
				"upload_time":       now,
			}).Error; err != nil {
				return fmt.Errorf("update reused file: %w", err)
			}
		}

		taskRemark := optionalString("复用旧结果，未重新生成")
		taskStatus := "SUCCESS"
		if !reuse {
			taskRemark = nil
			taskStatus = "PENDING"
		}
		task := model.GenerateTask{
			TaskNo:           fmt.Sprintf("GEN-%s-%04d", now.Format("20060102"), now.Nanosecond()%10000),
			FileID:           &file.ID,
			UploadUserID:     current.User.ID,
			TriggerType:      "UPLOAD",
			Status:           taskStatus,
			FileSnapshotName: fileHeader.Filename,
			FileSnapshotHash: hash,
			ReuseExisting:    reuse,
			TaskRemark:       taskRemark,
			ExpiresAt:        now.Add(30 * 24 * time.Hour),
		}
		if reuse {
			task.StartedAt = &now
			task.FinishedAt = &now
		}
		if err := tx.Create(&task).Error; err != nil {
			return fmt.Errorf("create task: %w", err)
		}

		if reuse {
			var generateRecord model.FileGenerateRecord
			if err := tx.Where("file_id = ?", file.ID).First(&generateRecord).Error; err != nil {
				return fmt.Errorf("query generate record: %w", err)
			}
			if err := tx.Model(&generateRecord).Updates(map[string]any{
				"total_status":      "SUCCESS",
				"last_generated_at": now,
			}).Error; err != nil {
				return fmt.Errorf("update generate record status: %w", err)
			}
			var latestItems []model.FileGenerateRecordItem
			if err := tx.Where("generate_record_id = ?", generateRecord.ID).Find(&latestItems).Error; err != nil {
				return fmt.Errorf("query latest items: %w", err)
			}
			for _, item := range latestItems {
				taskItem := model.GenerateTaskItem{
					TaskID:            task.ID,
					ItemType:          item.ItemType,
					Status:            item.ItemStatus,
					ResultObjectURL:   item.ResultObjectURL,
					StartedAt:         &now,
					FinishedAt:        &now,
					MaxAutoRetryCount: 3,
					RetryIntervalSec:  5,
				}
				if err := tx.Create(&taskItem).Error; err != nil {
					return fmt.Errorf("create task item for reused result: %w", err)
				}
			}
		} else {
			for _, itemType := range []string{"QUESTION", "KNOWLEDGE", "EXTENDED"} {
				taskItem := model.GenerateTaskItem{
					TaskID:            task.ID,
					ItemType:          itemType,
					Status:            "PENDING",
					MaxAutoRetryCount: 3,
					RetryIntervalSec:  5,
				}
				if err := tx.Create(&taskItem).Error; err != nil {
					return fmt.Errorf("create task item: %w", err)
				}
			}
		}

		response = map[string]any{
			"fileId":           file.ID,
			"sourceFileName":   fileHeader.Filename,
			"sourceFileHash":   hash,
			"reuseExisting":    reuse,
			"generateRecordId": file.ID,
			"taskId":           task.ID,
			"taskNo":           task.TaskNo,
			"taskStatus":       task.Status,
			"taskRemark":       derefString(task.TaskRemark),
		}
		if !reuse {
			payload, payloadErr := json.Marshal(GenerateEvent{TaskID: task.ID})
			if payloadErr != nil {
				return fmt.Errorf("marshal generate event: %w", payloadErr)
			}
			if err := a.redis.XAdd(ctx, &redis.XAddArgs{
				Stream: a.cfg.Redis.GenerateStream,
				Values: map[string]any{"payload": string(payload)},
			}).Err(); err != nil {
				return fmt.Errorf("push generate task to redis stream: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, normalizeErr(err)
	}
	return response, nil
}

func (a *App) DeleteFile(ctx context.Context, current *CurrentUser, fileID uint64, confirmText string) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	if confirmText != "DELETE" {
		return newError(http.StatusBadRequest, codeBadRequest, "确认文本不正确", nil)
	}
	return normalizeErr(a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var file model.LearningFile
		if err := tx.First(&file, fileID).Error; err != nil {
			return newError(http.StatusNotFound, codeNotFound, "文件不存在", err)
		}
		var preview model.FilePreviewRecord
		_ = tx.Where("file_id = ?", fileID).First(&preview).Error
		var generateRecord model.FileGenerateRecord
		_ = tx.Where("file_id = ?", fileID).First(&generateRecord).Error
		var items []model.FileGenerateRecordItem
		_ = tx.Where("generate_record_id = ?", generateRecord.ID).Find(&items).Error

		objectURLs := []string{file.SourceObjectURL}
		if preview.PreviewObjectURL != nil {
			objectURLs = append(objectURLs, *preview.PreviewObjectURL)
		}
		for _, item := range items {
			if item.ResultObjectURL != nil {
				objectURLs = append(objectURLs, *item.ResultObjectURL)
			}
		}
		for _, objectURL := range objectURLs {
			if objectURL == "" {
				continue
			}
			if err := a.storage.Delete(ctx, objectURL); err != nil {
				return newError(http.StatusInternalServerError, codeInternal, "删除 OSS 资源失败", err)
			}
		}
		if err := tx.Delete(&model.FileGenerateRecordItem{}, "generate_record_id = ?", generateRecord.ID).Error; err != nil {
			return fmt.Errorf("delete latest generate items: %w", err)
		}
		if generateRecord.ID > 0 {
			if err := tx.Delete(&generateRecord).Error; err != nil {
				return fmt.Errorf("delete latest generate record: %w", err)
			}
		}
		if preview.ID > 0 {
			if err := tx.Delete(&preview).Error; err != nil {
				return fmt.Errorf("delete preview record: %w", err)
			}
		}
		if err := tx.Delete(&file).Error; err != nil {
			return fmt.Errorf("delete file: %w", err)
		}
		if err := tx.Model(&model.GenerateTask{}).Where("file_id = ?", fileID).
			Updates(map[string]any{"file_id": nil, "file_deleted_snapshot": true}).Error; err != nil {
			return fmt.Errorf("update task snapshots: %w", err)
		}
		if err := tx.Model(&model.PreviewConversionTask{}).Where("file_id = ?", fileID).
			Update("file_id", nil).Error; err != nil {
			return fmt.Errorf("update preview snapshots: %w", err)
		}
		return nil
	}))
}

func (a *App) PreviewSource(ctx context.Context, current *CurrentUser, fileID uint64) (map[string]any, error) {
	file, err := a.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, err
	}
	var preview model.FilePreviewRecord
	if err := a.db.WithContext(ctx).Where("file_id = ?", fileID).First(&preview).Error; err != nil {
		return nil, newError(http.StatusNotFound, codeNotFound, "预览记录不存在", err)
	}
	if preview.PreviewMode == "DIRECT" {
		signed, err := a.storage.SignGetURL(ctx, file.SourceObjectURL, a.cfg.App.SignedURLTTL)
		if err != nil {
			return nil, newError(http.StatusInternalServerError, codeInternal, "生成预览地址失败", err)
		}
		return map[string]any{
			"fileId":         file.ID,
			"previewMode":    preview.PreviewMode,
			"previewStatus":  "SUCCESS",
			"sourceFileType": file.SourceFileType,
			"previewUrl":     signed,
			"expireAt":       formatTime(time.Now().Add(a.cfg.App.SignedURLTTL)),
			"renderType":     detectRenderType(file.SourceFileType),
			"downloadUrl":    fmt.Sprintf("/api/v1/files/%d/download-source", file.ID),
		}, nil
	}
	if preview.PreviewStatus == "SUCCESS" && preview.PreviewObjectURL != nil {
		signed, err := a.storage.SignGetURL(ctx, *preview.PreviewObjectURL, a.cfg.App.SignedURLTTL)
		if err != nil {
			return nil, newError(http.StatusInternalServerError, codeInternal, "生成预览地址失败", err)
		}
		return map[string]any{
			"fileId":         file.ID,
			"previewMode":    preview.PreviewMode,
			"previewStatus":  preview.PreviewStatus,
			"sourceFileType": file.SourceFileType,
			"previewUrl":     signed,
			"expireAt":       formatTime(time.Now().Add(a.cfg.App.SignedURLTTL)),
			"renderType":     "PDF_SCROLL",
			"downloadUrl":    fmt.Sprintf("/api/v1/files/%d/download-source", file.ID),
		}, nil
	}

	if preview.PreviewStatus == "PENDING" || preview.PreviewStatus == "FAIL" {
		if err := a.enqueuePreviewTask(ctx, current.User.ID, file); err != nil {
			return nil, err
		}
		_ = a.db.WithContext(ctx).Model(&preview).Update("preview_status", "PROCESSING").Error
	}
	return map[string]any{
		"fileId":         file.ID,
		"previewMode":    preview.PreviewMode,
		"previewStatus":  "PROCESSING",
		"sourceFileType": file.SourceFileType,
		"previewUrl":     nil,
		"expireAt":       nil,
		"renderType":     "PDF_SCROLL",
		"downloadUrl":    fmt.Sprintf("/api/v1/files/%d/download-source", file.ID),
		"message":        "预览文件正在生成中，请稍后刷新",
	}, nil
}

func (a *App) RetryPreviewConversion(ctx context.Context, current *CurrentUser, fileID uint64) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	var file model.LearningFile
	if err := a.db.WithContext(ctx).First(&file, fileID).Error; err != nil {
		return newError(http.StatusNotFound, codeNotFound, "文件不存在", err)
	}
	return a.enqueuePreviewTask(ctx, current.User.ID, file)
}

func (a *App) PreviewResult(ctx context.Context, current *CurrentUser, fileID uint64, itemType string) (map[string]any, error) {
	file, err := a.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, err
	}
	var generateRecord model.FileGenerateRecord
	if err := a.db.WithContext(ctx).Where("file_id = ?", file.ID).First(&generateRecord).Error; err != nil {
		return nil, newError(http.StatusNotFound, codeNotFound, "生成记录不存在", err)
	}
	var item model.FileGenerateRecordItem
	if err := a.db.WithContext(ctx).Where("generate_record_id = ? AND item_type = ?", generateRecord.ID, itemType).First(&item).Error; err != nil {
		return nil, newError(http.StatusNotFound, codeNotFound, "结果不存在", err)
	}
	if item.ResultObjectURL == nil {
		return map[string]any{
			"fileId":     file.ID,
			"itemType":   item.ItemType,
			"itemStatus": item.ItemStatus,
			"previewUrl": nil,
			"expireAt":   nil,
		}, nil
	}
	signed, err := a.storage.SignGetURL(ctx, *item.ResultObjectURL, a.cfg.App.SignedURLTTL)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "生成结果预览地址失败", err)
	}
	return map[string]any{
		"fileId":     file.ID,
		"itemType":   item.ItemType,
		"itemStatus": item.ItemStatus,
		"previewUrl": signed,
		"expireAt":   formatTime(time.Now().Add(a.cfg.App.SignedURLTTL)),
	}, nil
}

func (a *App) DownloadSource(ctx context.Context, current *CurrentUser, fileID uint64) (map[string]any, error) {
	file, err := a.loadAccessibleFile(ctx, current, fileID)
	if err != nil {
		return nil, err
	}
	signed, err := a.storage.SignGetURL(ctx, file.SourceObjectURL, a.cfg.App.SignedURLTTL)
	if err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "生成下载地址失败", err)
	}
	return map[string]any{
		"url":      signed,
		"expireAt": formatTime(time.Now().Add(a.cfg.App.SignedURLTTL)),
	}, nil
}

func (a *App) DownloadResult(ctx context.Context, current *CurrentUser, fileID uint64, itemType string) (map[string]any, error) {
	result, err := a.PreviewResult(ctx, current, fileID, itemType)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"url":      result["previewUrl"],
		"expireAt": result["expireAt"],
	}, nil
}

func (a *App) ListTasks(ctx context.Context, current *CurrentUser, req ListTaskRequest) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	tx := a.db.WithContext(ctx).Model(&model.GenerateTask{}).Where("upload_user_id = ?", current.User.ID)
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	var tasks []model.GenerateTask
	return pageQuery(ctx, tx, req.PageNo, req.PageSize, "created_at DESC", &tasks, func(task model.GenerateTask) map[string]any {
		return map[string]any{
			"id":                  task.ID,
			"taskNo":              task.TaskNo,
			"status":              task.Status,
			"triggerType":         task.TriggerType,
			"fileSnapshotName":    task.FileSnapshotName,
			"fileDeletedSnapshot": task.FileDeletedSnapshot,
			"startedAt":           formatTimePtr(task.StartedAt),
			"finishedAt":          formatTimePtr(task.FinishedAt),
			"lastErrorMessage":    derefString(task.LastErrorMessage),
			"reuseExisting":       task.ReuseExisting,
			"taskRemark":          derefString(task.TaskRemark),
		}
	})
}

func (a *App) GetTask(ctx context.Context, current *CurrentUser, taskID uint64) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	var task model.GenerateTask
	if err := a.db.WithContext(ctx).Where("id = ? AND upload_user_id = ?", taskID, current.User.ID).First(&task).Error; err != nil {
		return nil, newError(http.StatusNotFound, codeNotFound, "任务不存在", err)
	}
	var items []model.GenerateTaskItem
	if err := a.db.WithContext(ctx).Where("task_id = ?", task.ID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询任务子项失败", err)
	}
	dtoItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		dtoItems = append(dtoItems, map[string]any{
			"id":               item.ID,
			"itemType":         item.ItemType,
			"status":           item.Status,
			"autoRetryCount":   item.AutoRetryCount,
			"manualRetryCount": item.ManualRetryCount,
			"lastErrorMessage": derefString(item.LastErrorMessage),
			"resultObjectUrl":  item.ResultObjectURL,
		})
	}
	return map[string]any{
		"id":                  task.ID,
		"taskNo":              task.TaskNo,
		"status":              task.Status,
		"triggerType":         task.TriggerType,
		"fileSnapshotName":    task.FileSnapshotName,
		"fileDeletedSnapshot": task.FileDeletedSnapshot,
		"startedAt":           formatTimePtr(task.StartedAt),
		"finishedAt":          formatTimePtr(task.FinishedAt),
		"lastErrorMessage":    derefString(task.LastErrorMessage),
		"reuseExisting":       task.ReuseExisting,
		"taskRemark":          derefString(task.TaskRemark),
		"items":               dtoItems,
	}, nil
}

func (a *App) RetryTaskItem(ctx context.Context, current *CurrentUser, taskID uint64, taskItemID uint64) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	return normalizeErr(a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.GenerateTask
		if err := tx.Where("id = ? AND upload_user_id = ?", taskID, current.User.ID).First(&task).Error; err != nil {
			return newError(http.StatusNotFound, codeNotFound, "任务不存在", err)
		}
		var item model.GenerateTaskItem
		if err := tx.Where("id = ? AND task_id = ?", taskItemID, taskID).First(&item).Error; err != nil {
			return newError(http.StatusNotFound, codeNotFound, "任务子项不存在", err)
		}
		if item.Status != "FAIL" {
			return newError(http.StatusConflict, codeConflict, "仅失败子任务允许重试", nil)
		}
		if err := tx.Model(&item).Updates(map[string]any{
			"status":             "PENDING",
			"manual_retry_count": item.ManualRetryCount + 1,
			"last_error_message": nil,
			"last_error_code":    nil,
		}).Error; err != nil {
			return fmt.Errorf("reset task item: %w", err)
		}
		log := model.TaskRetryLog{
			BizType:       "GENERATE_ITEM",
			BizID:         item.ID,
			TaskID:        &taskID,
			RetryMode:     "MANUAL",
			RetryNo:       item.ManualRetryCount + 1,
			StatusBefore:  "FAIL",
			StatusAfter:   "PENDING",
			TriggerUserID: &current.User.ID,
		}
		if err := tx.Create(&log).Error; err != nil {
			return fmt.Errorf("create retry log: %w", err)
		}
		payload, err := json.Marshal(GenerateEvent{TaskID: taskID})
		if err != nil {
			return fmt.Errorf("marshal generate event: %w", err)
		}
		if err := a.redis.XAdd(ctx, &redis.XAddArgs{
			Stream: a.cfg.Redis.GenerateStream,
			Values: map[string]any{"payload": string(payload)},
		}).Err(); err != nil {
			return fmt.Errorf("push retry task to stream: %w", err)
		}
		return nil
	}))
}

func (a *App) ListNotifications(ctx context.Context, current *CurrentUser, req ListNotificationRequest) (map[string]any, error) {
	tx := a.db.WithContext(ctx).Model(&model.SystemNotification{}).Where("user_id = ?", current.User.ID)
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	if req.Type != "" {
		tx = tx.Where("type = ?", req.Type)
	}
	var list []model.SystemNotification
	return pageQuery(ctx, tx, req.PageNo, req.PageSize, "created_at DESC", &list, func(item model.SystemNotification) map[string]any {
		return notificationDTO(item)
	})
}

func (a *App) GetNotification(ctx context.Context, current *CurrentUser, id uint64) (map[string]any, error) {
	var record model.SystemNotification
	if err := a.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, current.User.ID).First(&record).Error; err != nil {
		return nil, newError(http.StatusNotFound, codeNotFound, "通知不存在", err)
	}
	now := time.Now()
	_ = a.db.WithContext(ctx).Model(&record).Updates(map[string]any{"status": "READ", "read_at": now}).Error
	record.Status = "READ"
	record.ReadAt = &now
	return notificationDTO(record), nil
}

func (a *App) MarkNotificationRead(ctx context.Context, current *CurrentUser, id uint64) error {
	if err := a.db.WithContext(ctx).Model(&model.SystemNotification{}).
		Where("id = ? AND user_id = ?", id, current.User.ID).
		Updates(map[string]any{"status": "READ", "read_at": time.Now()}).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "标记已读失败", err)
	}
	return nil
}

func (a *App) MarkNotificationsReadBatch(ctx context.Context, current *CurrentUser, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	if err := a.db.WithContext(ctx).Model(&model.SystemNotification{}).
		Where("user_id = ? AND id IN ?", current.User.ID, ids).
		Updates(map[string]any{"status": "READ", "read_at": time.Now()}).Error; err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "批量标记已读失败", err)
	}
	return nil
}

func (a *App) UnreadCount(ctx context.Context, current *CurrentUser) (map[string]any, error) {
	var count int64
	if err := a.db.WithContext(ctx).Model(&model.SystemNotification{}).
		Where("user_id = ? AND status = ?", current.User.ID, "UNREAD").
		Count(&count).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询未读数量失败", err)
	}
	return map[string]any{"unreadCount": count}, nil
}

func (a *App) ListUsers(ctx context.Context, current *CurrentUser, req ListUserRequest) (map[string]any, error) {
	if current.User.Role != "ADMIN" {
		return nil, newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	tx := a.db.WithContext(ctx).Model(&model.User{})
	if req.Email != "" {
		tx = tx.Where("email LIKE ?", "%"+req.Email+"%")
	}
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	var users []model.User
	return pageQuery(ctx, tx, req.PageNo, req.PageSize, "registered_at DESC", &users, func(user model.User) map[string]any {
		return map[string]any{
			"id":           user.ID,
			"email":        user.Email,
			"role":         user.Role,
			"status":       user.Status,
			"registeredAt": formatTime(user.RegisteredAt),
		}
	})
}

func (a *App) EnableUser(ctx context.Context, current *CurrentUser, id uint64) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	return normalizeErr(a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
			"status":      "ENABLED",
			"disabled_at": nil,
			"disabled_by": nil,
			"remark":      nil,
		}).Error; err != nil {
			return fmt.Errorf("enable user: %w", err)
		}
		return nil
	}))
}

func (a *App) DisableUser(ctx context.Context, current *CurrentUser, id uint64, remark string) error {
	if current.User.Role != "ADMIN" {
		return newError(http.StatusForbidden, codeForbidden, "无权限", nil)
	}
	return normalizeErr(a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
			"status":      "DISABLED",
			"disabled_at": now,
			"disabled_by": current.User.ID,
			"remark":      optionalString(remark),
		}).Error; err != nil {
			return fmt.Errorf("disable user: %w", err)
		}
		reason := "ACCOUNT_DISABLED"
		if err := tx.Model(&model.UserSession{}).Where("user_id = ? AND status = ?", id, "ACTIVE").Updates(map[string]any{
			"status":         "INVALIDATED",
			"invalidated_at": now,
			"invalid_reason": reason,
		}).Error; err != nil {
			return fmt.Errorf("invalidate user sessions: %w", err)
		}
		return nil
	}))
}

func (a *App) enqueuePreviewTask(ctx context.Context, userID uint64, file model.LearningFile) error {
	now := time.Now()
	return normalizeErr(a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		task := model.PreviewConversionTask{
			FileID:            &file.ID,
			RequestUserID:     userID,
			SourceFileType:    file.SourceFileType,
			Status:            "PENDING",
			MaxAutoRetryCount: 3,
			RetryIntervalSec:  5,
			ExpiresAt:         now.Add(30 * 24 * time.Hour),
		}
		if err := tx.Create(&task).Error; err != nil {
			return fmt.Errorf("create preview conversion task: %w", err)
		}
		payload, err := json.Marshal(PreviewEvent{ConversionTaskID: task.ID})
		if err != nil {
			return fmt.Errorf("marshal preview event: %w", err)
		}
		if err := a.redis.XAdd(ctx, &redis.XAddArgs{
			Stream: a.cfg.Redis.PreviewStream,
			Values: map[string]any{"payload": string(payload)},
		}).Err(); err != nil {
			return fmt.Errorf("push preview event: %w", err)
		}
		return nil
	}))
}

func (a *App) processPreviewTask(ctx context.Context, taskID uint64) error {
	var task model.PreviewConversionTask
	if err := a.db.WithContext(ctx).First(&task, taskID).Error; err != nil {
		return err
	}
	if task.FileID == nil {
		return nil
	}
	var file model.LearningFile
	if err := a.db.WithContext(ctx).First(&file, *task.FileID).Error; err != nil {
		return err
	}
	now := time.Now()
	_ = a.db.WithContext(ctx).Model(&task).Updates(map[string]any{"status": "PROCESSING", "started_at": now}).Error

	sourceURL, err := a.storage.SignGetURL(ctx, file.SourceObjectURL, a.cfg.App.SignedURLTTL)
	if err != nil {
		return a.failPreviewTask(ctx, task, fmt.Errorf("sign source url: %w", err))
	}
	pdfData, err := a.converter.ConvertToPDF(ctx, sourceURL, file.SourceFileType)
	if err != nil {
		return a.failPreviewTask(ctx, task, err)
	}
	objectURL, err := a.storage.Upload(ctx, platform.BuildObjectKey("preview", strings.TrimSuffix(file.SourceFileName, pathExt(file.SourceFileName))+".pdf"), "application/pdf", pdfData)
	if err != nil {
		return a.failPreviewTask(ctx, task, err)
	}
	return normalizeErr(a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		finished := time.Now()
		if err := tx.Model(&task).Updates(map[string]any{
			"status":             "SUCCESS",
			"preview_object_url": objectURL,
			"finished_at":        finished,
			"last_error_message": nil,
		}).Error; err != nil {
			return fmt.Errorf("update preview task success: %w", err)
		}
		if err := tx.Model(&model.FilePreviewRecord{}).Where("file_id = ?", file.ID).Updates(map[string]any{
			"preview_status":     "SUCCESS",
			"preview_object_url": objectURL,
			"last_success_at":    finished,
			"last_error_message": nil,
		}).Error; err != nil {
			return fmt.Errorf("update latest preview record: %w", err)
		}
		return tx.Create(&model.SystemNotification{
			UserID:     file.UploadUserID,
			Type:       "PREVIEW_CONVERSION_SUCCESS",
			Title:      "预览转换成功",
			Summary:    fmt.Sprintf("%s 的预览 PDF 已生成完成", file.SourceFileName),
			Status:     "UNREAD",
			TargetType: "PREVIEW_TASK",
			TargetID:   &task.ID,
			ExpiresAt:  finished.Add(30 * 24 * time.Hour),
		}).Error
	}))
}

func (a *App) failPreviewTask(ctx context.Context, task model.PreviewConversionTask, cause error) error {
	msg := cause.Error()
	now := time.Now()
	_ = a.db.WithContext(ctx).Model(&task).Updates(map[string]any{
		"status":             "FAIL",
		"finished_at":        now,
		"last_error_message": msg,
	}).Error
	if task.FileID != nil {
		_ = a.db.WithContext(ctx).Model(&model.FilePreviewRecord{}).Where("file_id = ?", *task.FileID).Updates(map[string]any{
			"preview_status":     "FAIL",
			"last_error_message": msg,
		}).Error
	}
	return cause
}

func (a *App) processGenerateTask(ctx context.Context, taskID uint64) error {
	var task model.GenerateTask
	if err := a.db.WithContext(ctx).First(&task, taskID).Error; err != nil {
		return err
	}
	if task.FileID == nil {
		return nil
	}
	var file model.LearningFile
	if err := a.db.WithContext(ctx).First(&file, *task.FileID).Error; err != nil {
		return err
	}
	var items []model.GenerateTaskItem
	if err := a.db.WithContext(ctx).Where("task_id = ?", taskID).Order("id ASC").Find(&items).Error; err != nil {
		return err
	}
	now := time.Now()
	_ = a.db.WithContext(ctx).Model(&task).Updates(map[string]any{"status": "PROCESSING", "started_at": now}).Error
	_ = a.db.WithContext(ctx).Model(&model.FileGenerateRecord{}).Where("file_id = ?", file.ID).Update("total_status", "PROCESSING").Error

	sourceText, err := a.extractSourceText(ctx, file)
	if err != nil {
		return a.failGenerateTask(ctx, task, items, err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(items))
	for _, item := range items {
		if item.Status == "SUCCESS" {
			continue
		}
		itemCopy := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			if itemErr := a.processGenerateTaskItem(ctx, task, file, itemCopy, sourceText); itemErr != nil {
				errCh <- itemErr
			}
		}()
	}
	wg.Wait()
	close(errCh)

	errList := make([]string, 0)
	for itemErr := range errCh {
		errList = append(errList, itemErr.Error())
	}
	sort.Strings(errList)

	var refreshed []model.GenerateTaskItem
	if err := a.db.WithContext(ctx).Where("task_id = ?", task.ID).Find(&refreshed).Error; err != nil {
		return err
	}
	status := aggregateTaskStatus(refreshed)
	finished := time.Now()
	update := map[string]any{
		"status":      status,
		"finished_at": finished,
	}
	if len(errList) > 0 {
		update["last_error_message"] = strings.Join(errList, "; ")
	}
	if err := a.db.WithContext(ctx).Model(&task).Updates(update).Error; err != nil {
		return err
	}
	_ = a.db.WithContext(ctx).Model(&model.FileGenerateRecord{}).Where("file_id = ?", file.ID).Updates(map[string]any{
		"total_status":      status,
		"last_generated_at": finished,
	}).Error
	lastError := ""
	if value, ok := update["last_error_message"].(string); ok {
		lastError = value
	}
	return a.notifyGenerateResult(ctx, task, file, status, lastError)
}

func (a *App) processGenerateTaskItem(ctx context.Context, task model.GenerateTask, file model.LearningFile, item model.GenerateTaskItem, sourceText string) error {
	now := time.Now()
	_ = a.db.WithContext(ctx).Model(&item).Updates(map[string]any{"status": "PROCESSING", "started_at": now}).Error
	html, err := a.ai.GenerateHTML(ctx, item.ItemType, sourceText)
	if err != nil {
		msg := err.Error()
		_ = a.db.WithContext(ctx).Model(&item).Updates(map[string]any{
			"status":             "FAIL",
			"finished_at":        time.Now(),
			"last_error_message": msg,
		}).Error
		return err
	}
	objectURL, err := a.storage.Upload(ctx, platform.BuildObjectKey("result", fmt.Sprintf("%d_%s.html", file.ID, strings.ToLower(item.ItemType))), "text/html; charset=utf-8", []byte(html))
	if err != nil {
		msg := err.Error()
		_ = a.db.WithContext(ctx).Model(&item).Updates(map[string]any{
			"status":             "FAIL",
			"finished_at":        time.Now(),
			"last_error_message": msg,
		}).Error
		return err
	}
	finished := time.Now()
	if err := a.db.WithContext(ctx).Model(&item).Updates(map[string]any{
		"status":             "SUCCESS",
		"result_object_url":  objectURL,
		"finished_at":        finished,
		"last_error_message": nil,
	}).Error; err != nil {
		return err
	}
	var latest model.FileGenerateRecord
	if err := a.db.WithContext(ctx).Where("file_id = ?", file.ID).First(&latest).Error; err != nil {
		return err
	}
	if err := a.db.WithContext(ctx).Model(&model.FileGenerateRecordItem{}).
		Where("generate_record_id = ? AND item_type = ?", latest.ID, item.ItemType).
		Updates(map[string]any{
			"item_status":        "SUCCESS",
			"result_object_url":  objectURL,
			"last_success_at":    finished,
			"last_error_message": nil,
		}).Error; err != nil {
		return err
	}
	return nil
}

func (a *App) failGenerateTask(ctx context.Context, task model.GenerateTask, items []model.GenerateTaskItem, err error) error {
	msg := err.Error()
	now := time.Now()
	_ = a.db.WithContext(ctx).Model(&task).Updates(map[string]any{
		"status":             "FAIL",
		"finished_at":        now,
		"last_error_message": msg,
	}).Error
	for _, item := range items {
		if item.Status != "SUCCESS" {
			_ = a.db.WithContext(ctx).Model(&item).Updates(map[string]any{
				"status":             "FAIL",
				"finished_at":        now,
				"last_error_message": msg,
			}).Error
		}
	}
	if task.FileID != nil {
		_ = a.db.WithContext(ctx).Model(&model.FileGenerateRecord{}).Where("file_id = ?", *task.FileID).Update("total_status", "FAIL").Error
	}
	return err
}

func (a *App) extractSourceText(ctx context.Context, file model.LearningFile) (string, error) {
	signed, err := a.storage.SignGetURL(ctx, file.SourceObjectURL, a.cfg.App.SignedURLTTL)
	if err != nil {
		return "", fmt.Errorf("sign source url: %w", err)
	}
	if isPlainText(file.SourceFileType) {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, signed, nil)
		if reqErr != nil {
			return "", fmt.Errorf("build text download request: %w", reqErr)
		}
		resp, respErr := a.httpClient.Do(req)
		if respErr != nil {
			return "", fmt.Errorf("download text source: %w", respErr)
		}
		defer resp.Body.Close()
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if readErr != nil {
			return "", fmt.Errorf("read text source: %w", readErr)
		}
		return string(body), nil
	}
	if strings.HasPrefix(file.SourceFileType, "image/") {
		return a.ai.OCRText(ctx, signed)
	}
	return a.parser.ExtractText(ctx, signed, file.SourceFileType)
}

func (a *App) notifyGenerateResult(ctx context.Context, task model.GenerateTask, file model.LearningFile, status string, lastError string) error {
	typ := "GENERATE_SUCCESS"
	title := "生成成功"
	summary := fmt.Sprintf("%s 的复习内容已全部生成完成", file.SourceFileName)
	if status == "PARTIAL_SUCCESS" {
		typ = "PARTIAL_SUCCESS"
		title = "部分生成成功"
		summary = fmt.Sprintf("%s 的部分结果已生成完成", file.SourceFileName)
	}
	if status == "FAIL" {
		typ = "GENERATE_FAIL"
		title = "生成失败"
		summary = fmt.Sprintf("%s 生成失败", file.SourceFileName)
	}
	content := summary
	if lastError != "" {
		content = summary + "\n失败原因：" + lastError
	}
	return a.db.WithContext(ctx).Create(&model.SystemNotification{
		UserID:             file.UploadUserID,
		Type:               typ,
		Title:              title,
		Summary:            summary,
		Content:            &content,
		Status:             "UNREAD",
		TargetType:         "GENERATE_TASK",
		TargetID:           &task.ID,
		TargetSnapshotName: &task.FileSnapshotName,
		ErrorSummary:       optionalString(lastError),
		ExpiresAt:          time.Now().Add(30 * 24 * time.Hour),
	}).Error
}

func (a *App) invalidateAllSessionsAndUpdatePassword(ctx context.Context, userID uint64, hash string, reason string) error {
	return a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

func (a *App) createSessionToken(ctx context.Context, tx *gorm.DB, user model.User, loginIP, userAgent string) (map[string]any, error) {
	sessionID := uuid.NewString()
	expireAt := time.Now().Add(a.cfg.Auth.TokenTTL)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  fmt.Sprintf("%d", user.ID),
		"jti":  sessionID,
		"role": user.Role,
		"exp":  expireAt.Unix(),
		"iss":  a.cfg.Auth.Issuer,
	})
	signed, err := token.SignedString([]byte(a.cfg.Auth.JWTSecret))
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

func (a *App) loadAccessibleFile(ctx context.Context, current *CurrentUser, fileID uint64) (model.LearningFile, error) {
	var file model.LearningFile
	if err := a.db.WithContext(ctx).First(&file, fileID).Error; err != nil {
		return model.LearningFile{}, newError(http.StatusNotFound, codeNotFound, "文件不存在", err)
	}
	if current.User.Role != "ADMIN" && !(file.Visibility == "PUBLIC" || file.UploadUserID == current.User.ID) {
		return model.LearningFile{}, newError(http.StatusForbidden, codeForbidden, "无权限访问该文件", nil)
	}
	return file, nil
}

func (a *App) fileListItem(ctx context.Context, file model.LearningFile) (map[string]any, error) {
	var category model.FileCategory
	if err := a.db.WithContext(ctx).First(&category, file.CategoryID).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询分类失败", err)
	}
	var user model.User
	if err := a.db.WithContext(ctx).First(&user, file.UploadUserID).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询上传者失败", err)
	}
	var generateRecord model.FileGenerateRecord
	if err := a.db.WithContext(ctx).Where("file_id = ?", file.ID).First(&generateRecord).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询生成记录失败", err)
	}
	return map[string]any{
		"id":                  file.ID,
		"sourceFileName":      file.SourceFileName,
		"sourceFileType":      file.SourceFileType,
		"sourceFileSize":      file.SourceFileSize,
		"categoryId":          file.CategoryID,
		"categoryName":        category.Name,
		"visibility":          file.Visibility,
		"uploadUserId":        file.UploadUserID,
		"uploadUserEmail":     user.Email,
		"uploadTime":          formatTime(file.UploadTime),
		"generateTotalStatus": generateRecord.TotalStatus,
	}, nil
}

func (a *App) checkAndIncreaseSendLimit(ctx context.Context, email string) error {
	windows := []struct {
		TTL   time.Duration
		Limit int
	}{
		{TTL: time.Minute, Limit: 1},
		{TTL: 5 * time.Minute, Limit: 3},
		{TTL: 3 * time.Hour, Limit: 5},
		{TTL: 24 * time.Hour, Limit: 7},
	}
	pipe := a.redis.TxPipeline()
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

func (a *App) emailCodeKey(bizType string, email string) string {
	return fmt.Sprintf("email:code:%s:%s", bizType, email)
}

func (a *App) consumeEmailCode(ctx context.Context, bizType, email, code string) error {
	key := a.emailCodeKey(bizType, email)
	data, err := a.redis.Get(ctx, key).Result()
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
		_ = a.redis.Del(ctx, key).Err()
		return newError(http.StatusBadRequest, codeBusiness, "验证码已过期", nil)
	}
	if record.CodeHash != sha256Hex(code) {
		record.AttemptCnt++
		if record.AttemptCnt >= 3 {
			_ = a.redis.Del(ctx, key).Err()
			return newError(http.StatusBadRequest, codeBusiness, "验证码错误次数过多，请重新获取", nil)
		}
		updated, _ := json.Marshal(record)
		_ = a.redis.Set(ctx, key, updated, time.Until(record.ExpireAt)).Err()
		return newError(http.StatusBadRequest, codeBusiness, "验证码错误", nil)
	}
	if err := a.redis.Del(ctx, key).Err(); err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "消费验证码失败", err)
	}
	return nil
}

func (a *App) checkLoginBan(ctx context.Context, email string) error {
	banKey := fmt.Sprintf("login:ban:%s", email)
	ttl, err := a.redis.TTL(ctx, banKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return newError(http.StatusInternalServerError, codeInternal, "检查登录风控失败", err)
	}
	if ttl > 0 {
		return newError(http.StatusTooManyRequests, codeTooManyRequests, fmt.Sprintf("登录受限，请 %d 秒后再试", int(ttl.Seconds())), nil)
	}
	return nil
}

func (a *App) increaseLoginFailure(ctx context.Context, email string) error {
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
	pipe := a.redis.TxPipeline()
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
		if err := a.redis.Set(ctx, fmt.Sprintf("login:ban:%s", email), "1", maxBan).Err(); err != nil {
			return newError(http.StatusInternalServerError, codeInternal, "写入登录封禁失败", err)
		}
	}
	return nil
}

func (a *App) clearLoginFailure(ctx context.Context, email string) error {
	keys := []string{
		fmt.Sprintf("login:ban:%s", email),
		fmt.Sprintf("login:fail:%s:%d", email, int(time.Hour.Seconds())),
		fmt.Sprintf("login:fail:%s:%d", email, int((3 * time.Hour).Seconds())),
		fmt.Sprintf("login:fail:%s:%d", email, int((24 * time.Hour).Seconds())),
	}
	if err := a.redis.Del(ctx, keys...).Err(); err != nil {
		return newError(http.StatusInternalServerError, codeInternal, "清理登录失败次数失败", err)
	}
	return nil
}

func pageQuery[T any](ctx context.Context, tx *gorm.DB, pageNo int, pageSize int, orderBy string, target *[]T, mapper func(T) map[string]any) (map[string]any, error) {
	if pageNo <= 0 {
		pageNo = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	var total int64
	if err := tx.WithContext(ctx).Count(&total).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "统计分页数据失败", err)
	}
	if err := tx.WithContext(ctx).Order(orderBy).Offset((pageNo - 1) * pageSize).Limit(pageSize).Find(target).Error; err != nil {
		return nil, newError(http.StatusInternalServerError, codeInternal, "查询分页数据失败", err)
	}
	list := make([]map[string]any, 0, len(*target))
	for _, item := range *target {
		list = append(list, mapper(item))
	}
	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}
	return map[string]any{
		"list":       list,
		"pageNo":     pageNo,
		"pageSize":   pageSize,
		"total":      total,
		"totalPages": totalPages,
	}, nil
}

func notificationDTO(item model.SystemNotification) map[string]any {
	content := ""
	if item.Content != nil {
		content = *item.Content
	}
	var targetTaskID any
	if item.TargetID != nil && item.TargetType == "GENERATE_TASK" {
		targetTaskID = *item.TargetID
	}
	return map[string]any{
		"id":           item.ID,
		"title":        item.Title,
		"summary":      item.Summary,
		"content":      content,
		"type":         item.Type,
		"status":       item.Status,
		"createdAt":    formatTime(item.CreatedAt),
		"targetTaskId": targetTaskID,
	}
}

func inviteCodeDTO(item model.InviteCode) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"code":           item.Code,
		"totalQuota":     item.TotalQuota,
		"remainingQuota": item.RemainingQuota,
		"remark":         derefString(item.Remark),
		"batchNo":        item.BatchNo,
		"status":         item.Status,
	}
}

func aggregateTaskStatus(items []model.GenerateTaskItem) string {
	successCnt := 0
	failCnt := 0
	for _, item := range items {
		switch item.Status {
		case "SUCCESS":
			successCnt++
		case "FAIL":
			failCnt++
		}
	}
	switch {
	case successCnt == len(items):
		return "SUCCESS"
	case failCnt == len(items):
		return "FAIL"
	case successCnt > 0 && failCnt > 0:
		return "PARTIAL_SUCCESS"
	default:
		return "PROCESSING"
	}
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000")
}

func formatTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

func optionalString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateQQEmail(email string) error {
	if !strings.HasSuffix(email, "@qq.com") || !strings.Contains(email, "@") {
		return newError(http.StatusBadRequest, codeBusiness, "当前版本仅支持 QQ 邮箱", nil)
	}
	return nil
}

func validatePasswordPair(password, confirmPassword string) error {
	if len(password) < 8 {
		return newError(http.StatusBadRequest, codeBusiness, "密码至少 8 位", nil)
	}
	if password != confirmPassword {
		return newError(http.StatusBadRequest, codeBusiness, "两次输入的密码不一致", nil)
	}
	return nil
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

func randomDigits(n int) (string, error) {
	const digits = "0123456789"
	return randomStringFromSet(n, digits)
}

func randomAlphaNum(n int) (string, error) {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	return randomStringFromSet(n, chars)
}

func randomStringFromSet(n int, charset string) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = charset[int(buf[i])%len(charset)]
	}
	return string(out), nil
}

func sha256Hex(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func sha256HexBytes(input []byte) string {
	sum := sha256.Sum256(input)
	return hex.EncodeToString(sum[:])
}

func detectPreviewMode(contentType string) string {
	switch {
	case strings.Contains(contentType, "officedocument"), strings.Contains(contentType, "msword"), strings.Contains(contentType, "presentation"):
		return "CONVERT_TO_PDF"
	default:
		return "DIRECT"
	}
}

func detectRenderType(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "IMAGE_VERTICAL"
	case isPlainText(contentType):
		return "MARKDOWN_RENDER"
	default:
		return "PDF_SCROLL"
	}
}

func isPlainText(contentType string) bool {
	return strings.Contains(contentType, "text/plain") || strings.Contains(contentType, "text/markdown") || strings.Contains(contentType, "application/json")
}

func pathExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}

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

func ContextCurrentUser(c *gin.Context) (*CurrentUser, bool) {
	value, ok := c.Get("current_user")
	if !ok {
		return nil, false
	}
	current, ok := value.(*CurrentUser)
	return current, ok
}

func MustUint64Param(c *gin.Context, name string) (uint64, error) {
	value := strings.TrimSpace(c.Param(name))
	if value == "" {
		return 0, newError(http.StatusBadRequest, codeBadRequest, "路径参数缺失", nil)
	}
	var id uint64
	if _, err := fmt.Sscanf(value, "%d", &id); err != nil {
		return 0, newError(http.StatusBadRequest, codeBadRequest, "路径参数格式错误", err)
	}
	return id, nil
}

func RawSQLDB(db *gorm.DB) (*sql.DB, error) {
	return db.DB()
}
