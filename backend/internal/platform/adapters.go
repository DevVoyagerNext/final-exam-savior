package platform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"final-exam-savior/backend/internal/config"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type CaptchaPayload struct {
	LotNumber     string `json:"lot_number" form:"lot_number"`
	CaptchaOutput string `json:"captcha_output" form:"captcha_output"`
	PassToken     string `json:"pass_token" form:"pass_token"`
	GenTime       string `json:"gen_time" form:"gen_time"`
	CaptchaID     string `json:"captcha_id" form:"captcha_id"`
}

type CaptchaValidator interface {
	Validate(ctx context.Context, payload CaptchaPayload) error
}

type ObjectStorage interface {
	Upload(ctx context.Context, objectKey string, contentType string, data []byte) (string, error)
	SignGetURL(ctx context.Context, objectURL string, expire time.Duration) (string, error)
	Delete(ctx context.Context, objectURL string) error
}

type AIClient interface {
	GenerateHTML(ctx context.Context, itemType string, sourceText string) (string, error)
	OCRText(ctx context.Context, imageURL string) (string, error)
}

type Parser interface {
	ExtractText(ctx context.Context, sourceURL, sourceType string) (string, error)
}

type Converter interface {
	ConvertToPDF(ctx context.Context, sourceURL, sourceType string) ([]byte, error)
}

type SMTPMailer struct {
	cfg config.SMTPConfig
}

func NewSMTPMailer(cfg config.SMTPConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

func (m *SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	if m.cfg.Host == "" || m.cfg.Username == "" || m.cfg.Password == "" || m.cfg.From == "" {
		return fmt.Errorf("smtp config is incomplete")
	}
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	msg := strings.Builder{}
	msg.WriteString(fmt.Sprintf("From: %s\r\n", m.cfg.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	msg.WriteString(body)
	return smtp.SendMail(addr, auth, m.cfg.From, []string{to}, []byte(msg.String()))
}

type GeetestValidator struct {
	cfg    config.GeetestConfig
	client *http.Client
}

func NewGeetestValidator(cfg config.GeetestConfig) *GeetestValidator {
	return &GeetestValidator{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (v *GeetestValidator) Validate(ctx context.Context, payload CaptchaPayload) error {
	if v.cfg.CaptchaID == "" || v.cfg.PrivateKey == "" {
		return fmt.Errorf("geetest config is incomplete")
	}
	mac := hmac.New(sha256.New, []byte(v.cfg.PrivateKey))
	_, _ = mac.Write([]byte(payload.LotNumber))
	signToken := hex.EncodeToString(mac.Sum(nil))

	form := url.Values{}
	form.Set("lot_number", payload.LotNumber)
	form.Set("captcha_output", payload.CaptchaOutput)
	form.Set("pass_token", payload.PassToken)
	form.Set("gen_time", payload.GenTime)
	form.Set("captcha_id", payload.CaptchaID)
	form.Set("sign_token", signToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.cfg.ValidateURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build geetest request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("call geetest validate: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Status  string `json:"status"`
		Result  string `json:"result"`
		Captcha string `json:"captcha_args"`
		Reason  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode geetest response: %w", err)
	}
	if result.Status != "success" || result.Result != "success" {
		if result.Reason == "" {
			result.Reason = "captcha validation failed"
		}
		return fmt.Errorf(result.Reason)
	}
	return nil
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

type OpenAICompatClient struct {
	cfg    config.AIConfig
	client *http.Client
}

func NewOpenAICompatClient(cfg config.AIConfig) *OpenAICompatClient {
	return &OpenAICompatClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *OpenAICompatClient) GenerateHTML(ctx context.Context, itemType string, sourceText string) (string, error) {
	prompt := fmt.Sprintf("你是期末复习助手。请基于以下学习材料生成一份可离线打开的完整 HTML 页面。目标类型：%s。要求输出完整 html 文档，仅输出 HTML，不要额外解释。\n\n学习材料：\n%s", itemType, sourceText)
	return c.chat(ctx, c.cfg.Model, []map[string]any{
		{
			"role":    "user",
			"content": prompt,
		},
	})
}

func (c *OpenAICompatClient) OCRText(ctx context.Context, imageURL string) (string, error) {
	model := c.cfg.OCRModel
	if model == "" {
		model = c.cfg.Model
	}
	return c.chat(ctx, model, []map[string]any{
		{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": "请对图片执行 OCR，输出尽量完整的纯文本，不要附加说明。"},
				{"type": "image_url", "image_url": map[string]string{"url": imageURL}},
			},
		},
	})
}

func (c *OpenAICompatClient) chat(ctx context.Context, model string, messages []map[string]any) (string, error) {
	if c.cfg.BaseURL == "" || c.cfg.APIKey == "" || model == "" {
		return "", fmt.Errorf("openai compatible config is incomplete")
	}
	payload := map[string]any{
		"model":    model,
		"messages": messages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal ai payload: %w", err)
	}

	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build ai request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call ai api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("ai api status %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode ai response: %w", err)
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("ai response is empty")
	}
	return result.Choices[0].Message.Content, nil
}

type HTTPParser struct {
	cfg    config.ParserConfig
	client *http.Client
}

func NewHTTPParser(cfg config.ParserConfig) *HTTPParser {
	return &HTTPParser{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (p *HTTPParser) ExtractText(ctx context.Context, sourceURL, sourceType string) (string, error) {
	if p.cfg.ExtractorURL == "" {
		return "", fmt.Errorf("parser extractor url is required")
	}
	payload := map[string]string{
		"sourceUrl":  sourceURL,
		"sourceType": sourceType,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal parser payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.ExtractorURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build parser request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call parser service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("parser service status %d: %s", resp.StatusCode, string(data))
	}
	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode parser response: %w", err)
	}
	if result.Text == "" {
		return "", fmt.Errorf("parser response text is empty")
	}
	return result.Text, nil
}

type HTTPConverter struct {
	cfg    config.PreviewConfig
	client *http.Client
}

func NewHTTPConverter(cfg config.PreviewConfig) *HTTPConverter {
	return &HTTPConverter{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *HTTPConverter) ConvertToPDF(ctx context.Context, sourceURL, sourceType string) ([]byte, error) {
	if c.cfg.ConverterURL == "" {
		return nil, fmt.Errorf("preview converter url is required")
	}
	payload := map[string]string{
		"sourceUrl":  sourceURL,
		"sourceType": sourceType,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal converter payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.ConverterURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build converter request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call converter service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("converter service status %d: %s", resp.StatusCode, string(data))
	}
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var result struct {
			PDFBase64   string `json:"pdfBase64"`
			DownloadURL string `json:"downloadUrl"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode converter response: %w", err)
		}
		if result.PDFBase64 != "" {
			data, err := base64.StdEncoding.DecodeString(result.PDFBase64)
			if err != nil {
				return nil, fmt.Errorf("decode converter pdfBase64: %w", err)
			}
			return data, nil
		}
		if result.DownloadURL != "" {
			return c.download(ctx, result.DownloadURL)
		}
		return nil, fmt.Errorf("converter response missing pdfBase64 or downloadUrl")
	}
	return io.ReadAll(resp.Body)
}

func (c *HTTPConverter) download(ctx context.Context, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build download request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download converted pdf: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("download converted pdf status %d: %s", resp.StatusCode, string(data))
	}
	return io.ReadAll(resp.Body)
}

func BuildObjectKey(prefix, fileName string) string {
	ts := time.Now().UTC().Format("20060102/150405")
	safeName := strings.ReplaceAll(fileName, " ", "_")
	return path.Join(prefix, ts, fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeName))
}

func ReadMultipartFile(header *multipart.FileHeader) ([]byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("open multipart file: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read multipart file: %w", err)
	}
	return data, nil
}
