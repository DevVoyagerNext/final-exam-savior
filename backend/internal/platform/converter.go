package platform

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"final-exam-savior/backend/internal/config"
)

type Converter interface {
	ConvertToPDF(ctx context.Context, sourceURL, sourceType string) ([]byte, error)
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
