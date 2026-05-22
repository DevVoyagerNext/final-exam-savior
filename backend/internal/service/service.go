package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"final-exam-savior/backend/internal/config"
	"final-exam-savior/backend/internal/dao"
	"final-exam-savior/backend/internal/model"
	"final-exam-savior/backend/internal/platform"
)

type Service struct {
	dao          *dao.DAO
	cfg          config.Config
	db           *gorm.DB
	redis        *redis.Client
	mailer       platform.Mailer
	captcha      platform.CaptchaValidator
	storage      platform.ObjectStorage
	localStorage *platform.LocalFileStorage
	ai           platform.AIClient
	httpClient   *http.Client
}

func New(ctx context.Context, cfg config.Config) (*Service, error) {
	store, err := dao.New(ctx, cfg)
	if err != nil {
		return nil, err
	}

	storage, localStorage, err := buildStorage(cfg)
	if err != nil {
		_ = store.Close()
		return nil, err
	}

	svc := &Service{
		dao:          store,
		cfg:          cfg,
		mailer:       platform.NewSMTPMailer(cfg.SMTP),
		captcha:      platform.NewGeetestValidator(cfg.Geetest),
		storage:      storage,
		localStorage: localStorage,
		ai:           platform.NewOpenAICompatClient(cfg.AI),
		httpClient:   &http.Client{Timeout: 2 * time.Minute},
	}

	if cfg.Database.AutoMigrate {
		if err := svc.dao.Gorm().AutoMigrate(model.AutoMigrateModels()...); err != nil {
			_ = store.Close()
			return nil, fmt.Errorf("auto migrate: %w", err)
		}
		if err := svc.dropObsoleteTables(); err != nil {
			_ = store.Close()
			return nil, fmt.Errorf("drop obsolete tables: %w", err)
		}
	}

	if err := svc.ensureSeeds(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	if err := svc.ensureRedisGroups(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	return svc, nil
}
func buildStorage(cfg config.Config) (platform.ObjectStorage, *platform.LocalFileStorage, error) {
	localStorage, localErr := buildOptionalLocalStorage(cfg)
	ossStorage, ossErr := buildOptionalOSSStorage(cfg)

	switch strings.ToLower(strings.TrimSpace(cfg.Storage.Mode)) {
	case "", "local":
		if localErr != nil {
			return nil, nil, fmt.Errorf("init local storage: %w", localErr)
		}
		if localStorage == nil {
			return nil, nil, fmt.Errorf("init local storage: local storage config is incomplete")
		}
		return platform.NewHybridStorage(localStorage, localStorage, ossStorage), localStorage, nil
	case "oss":
		if ossErr != nil {
			return nil, nil, ossErr
		}
		if ossStorage == nil {
			return nil, nil, fmt.Errorf("init oss storage: oss config is incomplete")
		}
		return platform.NewHybridStorage(ossStorage, localStorage, ossStorage), localStorage, nil
	default:
		return nil, nil, fmt.Errorf("unsupported storage mode: %s", cfg.Storage.Mode)
	}
}

func buildOptionalLocalStorage(cfg config.Config) (*platform.LocalFileStorage, error) {
	if strings.TrimSpace(cfg.Storage.LocalBasePath) == "" {
		return nil, nil
	}
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return nil, nil
	}
	return platform.NewLocalFileStorage(cfg.Storage, cfg.Auth.JWTSecret)
}

func buildOptionalOSSStorage(cfg config.Config) (*platform.OSSStorage, error) {
	if strings.TrimSpace(cfg.OSS.Endpoint) == "" ||
		strings.TrimSpace(cfg.OSS.Bucket) == "" ||
		strings.TrimSpace(cfg.OSS.AccessKeyID) == "" ||
		strings.TrimSpace(cfg.OSS.AccessKeySecret) == "" {
		return nil, nil
	}
	return platform.NewOSSStorage(cfg.OSS)
}

func (s *Service) DB() *gorm.DB { return s.dao.Gorm() }

func (s *Service) ensureSeeds(ctx context.Context) error {
	var count int64
	if err := s.dao.Gorm().WithContext(ctx).Model(&model.FileCategory{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count categories: %w", err)
	}
	if count == 0 {
		defaultCategory := model.FileCategory{
			Name:      s.cfg.App.DefaultCategoryName,
			IsBuiltin: true,
			Status:    "ENABLED",
			SortNo:    0,
		}
		if err := s.dao.Gorm().WithContext(ctx).Create(&defaultCategory).Error; err != nil {
			return fmt.Errorf("create default category: %w", err)
		}
	}

	var admin model.User
	err := s.dao.Gorm().WithContext(ctx).Where("email = ?", s.cfg.App.DefaultAdminEmail).First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) && s.cfg.App.DefaultAdminEmail != "" && s.cfg.App.DefaultAdminPass != "" {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(s.cfg.App.DefaultAdminPass), bcrypt.DefaultCost)
		if hashErr != nil {
			return fmt.Errorf("hash default admin password: %w", hashErr)
		}
		now := time.Now()
		admin = model.User{
			Email:        strings.ToLower(strings.TrimSpace(s.cfg.App.DefaultAdminEmail)),
			PasswordHash: string(hash),
			Role:         "ADMIN",
			Status:       "ENABLED",
			RegisteredAt: now,
		}
		if createErr := s.dao.Gorm().WithContext(ctx).Create(&admin).Error; createErr != nil {
			return fmt.Errorf("create default admin: %w", createErr)
		}
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query default admin: %w", err)
	}
	return nil
}
func (s *Service) ensureRedisGroups(ctx context.Context) error {
	for _, stream := range []string{s.cfg.Redis.GenerateStream} {
		err := s.dao.Redis().XGroupCreateMkStream(ctx, stream, s.cfg.Redis.ConsumerGroup, "$").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			return fmt.Errorf("create redis group %s: %w", stream, err)
		}
	}
	return nil
}

func (s *Service) dropObsoleteTables() error {
	return s.dao.Gorm().Migrator().DropTable("file_preview_records", "preview_conversion_tasks")
}

func (s *Service) Close() error {
	return s.dao.Close()
}
func (s *Service) ResolveLocalObjectPath(objectKey string, expireAt string, signature string) (string, error) {
	if s.localStorage == nil {
		return "", newError(http.StatusNotFound, codeNotFound, "本地存储未启用", nil)
	}
	expireAtUnix, err := strconv.ParseInt(strings.TrimSpace(expireAt), 10, 64)
	if err != nil {
		return "", newError(http.StatusBadRequest, codeBadRequest, "本地文件访问参数错误", err)
	}
	fullPath, err := s.localStorage.VerifyAndResolve(strings.TrimPrefix(objectKey, "/"), expireAtUnix, signature)
	if err != nil {
		return "", newError(http.StatusForbidden, codeForbidden, "本地文件访问无效或已过期", err)
	}
	return fullPath, nil
}
