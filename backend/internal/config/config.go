package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Auth     AuthConfig     `mapstructure:"auth"`
	SMTP     SMTPConfig     `mapstructure:"smtp"`
	OSS      OSSConfig      `mapstructure:"oss"`
	AI       AIConfig       `mapstructure:"ai"`
	Geetest  GeetestConfig  `mapstructure:"geetest"`
	App      AppConfig      `mapstructure:"app"`
}

type ServerConfig struct {
	Addr         string        `mapstructure:"addr"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type DatabaseConfig struct {
	DSN             string `mapstructure:"dsn"`
	AutoMigrate     bool   `mapstructure:"auto_migrate"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime_minutes"`
}

type RedisConfig struct {
	Addr                 string        `mapstructure:"addr"`
	Password             string        `mapstructure:"password"`
	DB                   int           `mapstructure:"db"`
	GenerateStream       string        `mapstructure:"generate_stream"`
	PreviewStream        string        `mapstructure:"preview_stream"`
	ConsumerGroup        string        `mapstructure:"consumer_group"`
	ConsumerName         string        `mapstructure:"consumer_name"`
	BlockTimeout         time.Duration `mapstructure:"block_timeout"`
	VisibilityTimeoutSec int64         `mapstructure:"visibility_timeout_sec"`
}

type StorageConfig struct {
	Mode          string `mapstructure:"mode"`
	LocalBasePath string `mapstructure:"local_base_path"`
	PublicBaseURL string `mapstructure:"public_base_url"`
}

type AuthConfig struct {
	JWTSecret       string        `mapstructure:"jwt_secret"`
	Issuer          string        `mapstructure:"issuer"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
}

type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

type OSSConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	BaseURL         string `mapstructure:"base_url"`
}

type AIConfig struct {
	BaseURL  string        `mapstructure:"base_url"`
	APIKey   string        `mapstructure:"api_key"`
	Model    string        `mapstructure:"model"`
	OCRModel string        `mapstructure:"ocr_model"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

type GeetestConfig struct {
	CaptchaID   string        `mapstructure:"captcha_id"`
	PrivateKey  string        `mapstructure:"private_key"`
	ValidateURL string        `mapstructure:"validate_url"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type AppConfig struct {
	Env                 string        `mapstructure:"env"`
	DefaultAdminEmail   string        `mapstructure:"default_admin_email"`
	DefaultAdminPass    string        `mapstructure:"default_admin_password"`
	DefaultCategoryName string        `mapstructure:"default_category_name"`
	SignedURLTTL        time.Duration `mapstructure:"signed_url_ttl"`
}

func Load() (Config, error) {
	// 尝试从多个可能的路径加载 .env 文件，按顺序尝试，不报错
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load("../../.env")

	v := viper.New()
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	cfg = applyRuntimeValues(v, cfg)

	if cfg.Database.DSN == "" {
		return Config{}, fmt.Errorf("database dsn is required")
	}
	if cfg.Redis.Addr == "" {
		return Config{}, fmt.Errorf("redis addr is required")
	}
	if strings.EqualFold(cfg.Storage.Mode, "local") && cfg.Storage.LocalBasePath == "" {
		return Config{}, fmt.Errorf("storage local base path is required when storage mode is local")
	}
	if cfg.Auth.JWTSecret == "" {
		return Config{}, fmt.Errorf("auth jwt secret is required")
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.read_timeout", 10*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)
	v.SetDefault("server.idle_timeout", 60*time.Second)

	v.SetDefault("database.auto_migrate", true)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.max_open_conns", 20)
	v.SetDefault("database.conn_max_lifetime_minutes", 30)

	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.generate_stream", "fes:stream:generate")
	v.SetDefault("redis.preview_stream", "fes:stream:preview")
	v.SetDefault("redis.consumer_group", "fes-workers")
	v.SetDefault("redis.consumer_name", "worker-1")
	v.SetDefault("redis.block_timeout", 3*time.Second)
	v.SetDefault("redis.visibility_timeout_sec", 60)

	v.SetDefault("storage.mode", "local")
	v.SetDefault("storage.local_base_path", "./data/storage")
	v.SetDefault("storage.public_base_url", "http://127.0.0.1:8080/storage/local")

	v.SetDefault("auth.issuer", "final-exam-savior")
	v.SetDefault("auth.access_token_ttl", 2*time.Hour)
	v.SetDefault("auth.refresh_token_ttl", 30*24*time.Hour)

	v.SetDefault("smtp.port", 587)
	v.SetDefault("smtp.from", "")

	v.SetDefault("ai.timeout", 5*time.Minute)
	v.SetDefault("geetest.validate_url", "https://gcaptcha4.geetest.com/validate")
	v.SetDefault("geetest.timeout", 8*time.Second)
	v.SetDefault("preview.timeout", 2*time.Minute)
	v.SetDefault("parser.timeout", 2*time.Minute)

	v.SetDefault("app.env", "dev")
	v.SetDefault("app.default_category_name", "默认分类")
	v.SetDefault("app.signed_url_ttl", 10*time.Minute)
}

func applyRuntimeValues(v *viper.Viper, cfg Config) Config {
	cfg.Server.Addr = v.GetString("server.addr")
	cfg.Server.ReadTimeout = v.GetDuration("server.read_timeout")
	cfg.Server.WriteTimeout = v.GetDuration("server.write_timeout")
	cfg.Server.IdleTimeout = v.GetDuration("server.idle_timeout")

	cfg.Database.DSN = firstNonEmpty(os.Getenv("APP_DATABASE_DSN"), v.GetString("database.dsn"))
	cfg.Database.AutoMigrate = v.GetBool("database.auto_migrate")
	cfg.Database.MaxIdleConns = v.GetInt("database.max_idle_conns")
	cfg.Database.MaxOpenConns = v.GetInt("database.max_open_conns")
	cfg.Database.ConnMaxLifetime = v.GetInt("database.conn_max_lifetime_minutes")

	cfg.Redis.Addr = firstNonEmpty(os.Getenv("APP_REDIS_ADDR"), v.GetString("redis.addr"))
	cfg.Redis.Password = firstNonEmpty(os.Getenv("APP_REDIS_PASSWORD"), v.GetString("redis.password"))
	cfg.Redis.DB = v.GetInt("redis.db")
	cfg.Redis.GenerateStream = v.GetString("redis.generate_stream")
	cfg.Redis.PreviewStream = v.GetString("redis.preview_stream")
	cfg.Redis.ConsumerGroup = v.GetString("redis.consumer_group")
	cfg.Redis.ConsumerName = v.GetString("redis.consumer_name")
	cfg.Redis.BlockTimeout = v.GetDuration("redis.block_timeout")
	cfg.Redis.VisibilityTimeoutSec = v.GetInt64("redis.visibility_timeout_sec")

	cfg.Storage.Mode = firstNonEmpty(os.Getenv("APP_STORAGE_MODE"), v.GetString("storage.mode"))
	cfg.Storage.LocalBasePath = firstNonEmpty(os.Getenv("APP_STORAGE_LOCAL_BASE_PATH"), v.GetString("storage.local_base_path"))
	cfg.Storage.PublicBaseURL = firstNonEmpty(os.Getenv("APP_STORAGE_PUBLIC_BASE_URL"), v.GetString("storage.public_base_url"))

	cfg.Auth.JWTSecret = firstNonEmpty(os.Getenv("APP_AUTH_JWT_SECRET"), v.GetString("auth.jwt_secret"))
	cfg.Auth.Issuer = v.GetString("auth.issuer")
	cfg.Auth.AccessTokenTTL = v.GetDuration("auth.access_token_ttl")
	cfg.Auth.RefreshTokenTTL = v.GetDuration("auth.refresh_token_ttl")
	if cfg.Auth.AccessTokenTTL <= 0 {
		cfg.Auth.AccessTokenTTL = v.GetDuration("auth.token_ttl")
	}
	if cfg.Auth.RefreshTokenTTL <= 0 {
		cfg.Auth.RefreshTokenTTL = 30 * 24 * time.Hour
	}

	cfg.SMTP.Host = firstNonEmpty(os.Getenv("APP_SMTP_HOST"), v.GetString("smtp.host"))
	cfg.SMTP.Port = v.GetInt("smtp.port")
	cfg.SMTP.Username = firstNonEmpty(os.Getenv("APP_SMTP_USERNAME"), v.GetString("smtp.username"))
	cfg.SMTP.Password = firstNonEmpty(os.Getenv("APP_SMTP_PASSWORD"), v.GetString("smtp.password"))
	cfg.SMTP.From = firstNonEmpty(os.Getenv("APP_SMTP_FROM"), v.GetString("smtp.from"))

	cfg.OSS.Endpoint = firstNonEmpty(os.Getenv("APP_OSS_ENDPOINT"), v.GetString("oss.endpoint"))
	cfg.OSS.Bucket = firstNonEmpty(os.Getenv("APP_OSS_BUCKET"), v.GetString("oss.bucket"))
	cfg.OSS.AccessKeyID = firstNonEmpty(os.Getenv("APP_OSS_ACCESS_KEY_ID"), v.GetString("oss.access_key_id"))
	cfg.OSS.AccessKeySecret = firstNonEmpty(os.Getenv("APP_OSS_ACCESS_KEY_SECRET"), v.GetString("oss.access_key_secret"))
	cfg.OSS.BaseURL = firstNonEmpty(os.Getenv("APP_OSS_BASE_URL"), v.GetString("oss.base_url"))

	cfg.AI.BaseURL = firstNonEmpty(os.Getenv("APP_AI_BASE_URL"), v.GetString("ai.base_url"))
	cfg.AI.APIKey = firstNonEmpty(os.Getenv("APP_AI_API_KEY"), v.GetString("ai.api_key"))
	cfg.AI.Model = firstNonEmpty(os.Getenv("APP_AI_MODEL"), v.GetString("ai.model"))
	cfg.AI.OCRModel = firstNonEmpty(os.Getenv("APP_AI_OCR_MODEL"), v.GetString("ai.ocr_model"))
	cfg.AI.Timeout = v.GetDuration("ai.timeout")

	cfg.Geetest.CaptchaID = firstNonEmpty(os.Getenv("APP_GEETEST_CAPTCHA_ID"), v.GetString("geetest.captcha_id"))
	cfg.Geetest.PrivateKey = firstNonEmpty(os.Getenv("APP_GEETEST_PRIVATE_KEY"), v.GetString("geetest.private_key"))
	cfg.Geetest.ValidateURL = firstNonEmpty(os.Getenv("APP_GEETEST_VALIDATE_URL"), v.GetString("geetest.validate_url"))
	cfg.Geetest.Timeout = v.GetDuration("geetest.timeout")

	cfg.App.Env = firstNonEmpty(os.Getenv("APP_APP_ENV"), v.GetString("app.env"))
	cfg.App.DefaultAdminEmail = firstNonEmpty(os.Getenv("APP_APP_DEFAULT_ADMIN_EMAIL"), v.GetString("app.default_admin_email"))
	cfg.App.DefaultAdminPass = firstNonEmpty(os.Getenv("APP_APP_DEFAULT_ADMIN_PASSWORD"), v.GetString("app.default_admin_password"))
	cfg.App.DefaultCategoryName = firstNonEmpty(os.Getenv("APP_APP_DEFAULT_CATEGORY_NAME"), v.GetString("app.default_category_name"))
	cfg.App.SignedURLTTL = v.GetDuration("app.signed_url_ttl")
	return cfg
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
