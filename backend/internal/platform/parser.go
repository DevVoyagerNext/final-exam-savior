package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"final-exam-savior/backend/internal/config"
)

type Parser interface {
	ExtractText(ctx context.Context, sourceURL, sourceType string) (string, error)
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
