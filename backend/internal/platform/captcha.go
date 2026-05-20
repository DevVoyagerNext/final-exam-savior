package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	SignToken     string `json:"sign_token" form:"sign_token"`
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
	if payload.CaptchaID == "" {
		return fmt.Errorf("missing captcha_id")
	}
	if payload.CaptchaID != v.cfg.CaptchaID {
		log.Printf("[GEETEST] captcha_id mismatch request=%s config=%s", payload.CaptchaID, v.cfg.CaptchaID)
		return fmt.Errorf("captcha_id mismatch")
	}

	signToken := payload.SignToken
	if signToken == "" {
		mac := hmac.New(sha256.New, []byte(v.cfg.PrivateKey))
		_, _ = mac.Write([]byte(payload.LotNumber))
		signToken = hex.EncodeToString(mac.Sum(nil))
	}

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
		log.Printf("[GEETEST] validate request failed captcha_id=%s lot_number=%s err=%v", payload.CaptchaID, payload.LotNumber, err)
		return fmt.Errorf("call geetest validate: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[GEETEST] read response failed captcha_id=%s lot_number=%s err=%v", payload.CaptchaID, payload.LotNumber, err)
		return fmt.Errorf("read geetest response: %w", err)
	}

	var result struct {
		Status      string          `json:"status"`
		Result      string          `json:"result"`
		CaptchaArgs json.RawMessage `json:"captcha_args"`
		Reason      string          `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[GEETEST] decode response failed captcha_id=%s lot_number=%s err=%v raw=%s", payload.CaptchaID, payload.LotNumber, err, string(body))
		return fmt.Errorf("decode geetest response: %w", err)
	}
	if result.Status != "success" || result.Result != "success" {
		if result.Reason == "" {
			result.Reason = "captcha validation failed"
		}
		log.Printf("[GEETEST] validate failed captcha_id=%s lot_number=%s status=%s result=%s reason=%s captcha_args=%s has_sign_token=%t",
			payload.CaptchaID,
			payload.LotNumber,
			result.Status,
			result.Result,
			result.Reason,
			string(result.CaptchaArgs),
			payload.SignToken != "",
		)
		return fmt.Errorf(result.Reason)
	}
	log.Printf("[GEETEST] validate success captcha_id=%s lot_number=%s captcha_args=%s has_sign_token=%t", payload.CaptchaID, payload.LotNumber, string(result.CaptchaArgs), payload.SignToken != "")
	return nil
}
