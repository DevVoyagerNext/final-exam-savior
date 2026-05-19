package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"final-exam-savior/backend/internal/config"
)

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
