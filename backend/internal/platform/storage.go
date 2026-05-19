package platform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"final-exam-savior/backend/internal/config"
)

type ObjectStorage interface {
	Upload(ctx context.Context, objectKey string, contentType string, data []byte) (string, error)
	SignGetURL(ctx context.Context, objectURL string, expire time.Duration) (string, error)
	Delete(ctx context.Context, objectURL string) error
}
type OSSStorage struct {
	cfg    config.OSSConfig
	bucket *oss.Bucket
}
type LocalFileStorage struct {
	cfg       config.StorageConfig
	signKey   []byte
	publicURL string
}

func NewOSSStorage(cfg config.OSSConfig) (*OSSStorage, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return nil, fmt.Errorf("oss config is incomplete")
	}
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create oss client: %w", err)
	}
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("open oss bucket: %w", err)
	}
	return &OSSStorage{cfg: cfg, bucket: bucket}, nil
}
func NewLocalFileStorage(cfg config.StorageConfig, signSecret string) (*LocalFileStorage, error) {
	if cfg.LocalBasePath == "" {
		return nil, fmt.Errorf("local storage base path is required")
	}
	if signSecret == "" {
		return nil, fmt.Errorf("local storage sign secret is required")
	}
	if err := os.MkdirAll(cfg.LocalBasePath, 0o755); err != nil {
		return nil, fmt.Errorf("create local storage base path: %w", err)
	}
	publicURL := strings.TrimRight(cfg.PublicBaseURL, "/")
	if publicURL == "" {
		publicURL = "http://127.0.0.1:8080/storage/local"
	}
	return &LocalFileStorage{
		cfg:       cfg,
		signKey:   []byte(signSecret),
		publicURL: publicURL,
	}, nil
}
func (s *OSSStorage) Upload(_ context.Context, objectKey string, contentType string, data []byte) (string, error) {
	if err := s.bucket.PutObject(objectKey, bytes.NewReader(data), oss.ContentType(contentType)); err != nil {
		return "", fmt.Errorf("upload object %s: %w", objectKey, err)
	}
	if s.cfg.BaseURL != "" {
		return strings.TrimRight(s.cfg.BaseURL, "/") + "/" + objectKey, nil
	}
	return fmt.Sprintf("https://%s.%s/%s", s.cfg.Bucket, s.cfg.Endpoint, objectKey), nil
}
func (s *OSSStorage) SignGetURL(_ context.Context, objectURL string, expire time.Duration) (string, error) {
	objectKey, err := s.objectKey(objectURL)
	if err != nil {
		return "", err
	}
	signed, err := s.bucket.SignURL(objectKey, http.MethodGet, int64(expire.Seconds()))
	if err != nil {
		return "", fmt.Errorf("sign oss url: %w", err)
	}
	return signed, nil
}
func (s *OSSStorage) Delete(_ context.Context, objectURL string) error {
	objectKey, err := s.objectKey(objectURL)
	if err != nil {
		return err
	}
	if err := s.bucket.DeleteObject(objectKey); err != nil {
		return fmt.Errorf("delete object %s: %w", objectKey, err)
	}
	return nil
}
func (s *OSSStorage) objectKey(objectURL string) (string, error) {
	if objectURL == "" {
		return "", fmt.Errorf("object url is empty")
	}
	u, err := url.Parse(objectURL)
	if err != nil {
		return "", fmt.Errorf("parse object url: %w", err)
	}
	return strings.TrimPrefix(u.Path, "/"), nil
}
func (s *LocalFileStorage) Upload(_ context.Context, objectKey string, _ string, data []byte) (string, error) {
	fullPath := filepath.Join(s.cfg.LocalBasePath, filepath.FromSlash(objectKey))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", fmt.Errorf("create local object dir: %w", err)
	}
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write local object: %w", err)
	}
	return "local://" + objectKey, nil
}
func (s *LocalFileStorage) SignGetURL(_ context.Context, objectURL string, expire time.Duration) (string, error) {
	objectKey, err := s.objectKey(objectURL)
	if err != nil {
		return "", err
	}
	expireAt := time.Now().Add(expire).Unix()
	sig := s.sign(objectKey, expireAt)
	return fmt.Sprintf("%s/%s?exp=%d&sig=%s", s.publicURL, objectKey, expireAt, sig), nil
}
func (s *LocalFileStorage) Delete(_ context.Context, objectURL string) error {
	objectKey, err := s.objectKey(objectURL)
	if err != nil {
		return err
	}
	fullPath := filepath.Join(s.cfg.LocalBasePath, filepath.FromSlash(objectKey))
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete local object: %w", err)
	}
	return nil
}
func (s *LocalFileStorage) VerifyAndResolve(objectKey string, expireAtUnix int64, signature string) (string, error) {
	if objectKey == "" {
		return "", fmt.Errorf("object key is empty")
	}
	if expireAtUnix <= time.Now().Unix() {
		return "", fmt.Errorf("signed local url is expired")
	}
	expected := s.sign(objectKey, expireAtUnix)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return "", fmt.Errorf("invalid local storage signature")
	}
	fullPath := filepath.Join(s.cfg.LocalBasePath, filepath.FromSlash(objectKey))
	if _, err := os.Stat(fullPath); err != nil {
		return "", fmt.Errorf("local object not found: %w", err)
	}
	return fullPath, nil
}
func (s *LocalFileStorage) objectKey(objectURL string) (string, error) {
	if strings.HasPrefix(objectURL, "local://") {
		return strings.TrimPrefix(objectURL, "local://"), nil
	}
	u, err := url.Parse(objectURL)
	if err != nil {
		return "", fmt.Errorf("parse local object url: %w", err)
	}
	if u.Scheme == "local" {
		return strings.TrimPrefix(u.Opaque, "/"), nil
	}
	pathValue := strings.TrimPrefix(u.Path, "/")
	if pathValue == "" {
		return "", fmt.Errorf("local object key is empty")
	}
	return pathValue, nil
}
func (s *LocalFileStorage) sign(objectKey string, expireAtUnix int64) string {
	mac := hmac.New(sha256.New, s.signKey)
	_, _ = mac.Write([]byte(objectKey))
	_, _ = mac.Write([]byte("|"))
	_, _ = mac.Write([]byte(strconv.FormatInt(expireAtUnix, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}
