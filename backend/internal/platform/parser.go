package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"final-exam-savior/backend/internal/config"
)

type Parser interface {
	ExtractText(ctx context.Context, sourceURL, sourceType string) (string, error)
}

type HTTPParser struct {
	cfg    config.ParserConfig
	client *http.Client
}

type DocstreamParser struct {
	cfg    config.ParserConfig
	client *http.Client
}

type HybridParser struct {
	remote *HTTPParser
	local  *DocstreamParser
}

func NewHTTPParser(cfg config.ParserConfig) *HTTPParser {
	return &HTTPParser{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func NewDocstreamParser(cfg config.ParserConfig) *DocstreamParser {
	return &DocstreamParser{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func NewParser(cfg config.ParserConfig) Parser {
	return &HybridParser{
		remote: NewHTTPParser(cfg),
		local:  NewDocstreamParser(cfg),
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

func (p *DocstreamParser) ExtractText(ctx context.Context, sourceURL, sourceType string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("build parser download request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download parser source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("download parser source status %d: %s", resp.StatusCode, string(data))
	}

	tempFile, err := os.CreateTemp("", "docstream-*"+guessParserExt(sourceURL, sourceType))
	if err != nil {
		return "", fmt.Errorf("create temp parser file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		tempFile.Close()
		return "", fmt.Errorf("write temp parser file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("close temp parser file: %w", err)
	}

	output, err := p.runLocalParser(ctx, tempPath, sourceType)
	text := strings.TrimSpace(output)
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return "", fmt.Errorf("local parser extract failed: %s", text)
	}
	if text == "" {
		return "", fmt.Errorf("local parser extracted empty text")
	}
	return text, nil
}

func (p *DocstreamParser) runLocalParser(ctx context.Context, tempPath string, sourceType string) (string, error) {
	scriptPath, err := filepath.Abs(filepath.Join("scripts", "local_parser.js"))
	if err != nil {
		return "", fmt.Errorf("resolve local parser script path: %w", err)
	}
	cmd := exec.CommandContext(ctx, "node", scriptPath, tempPath, sourceType)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (p *HybridParser) ExtractText(ctx context.Context, sourceURL, sourceType string) (string, error) {
	text, err := p.local.ExtractText(ctx, sourceURL, sourceType)
	if err == nil {
		return text, nil
	}
	if strings.TrimSpace(p.remote.cfg.ExtractorURL) == "" {
		return "", err
	}
	remoteText, remoteErr := p.remote.ExtractText(ctx, sourceURL, sourceType)
	if remoteErr != nil {
		return "", fmt.Errorf("docstream parser failed: %v; remote parser failed: %w", err, remoteErr)
	}
	return remoteText, nil
}

func guessParserExt(sourceURL, sourceType string) string {
	if parsed, err := url.Parse(sourceURL); err == nil {
		if ext := strings.ToLower(path.Ext(parsed.Path)); ext != "" {
			return ext
		}
	}
	switch {
	case strings.Contains(sourceType, "application/pdf"):
		return ".pdf"
	case strings.Contains(sourceType, "application/msword"):
		return ".doc"
	case strings.Contains(sourceType, "wordprocessingml.document"):
		return ".docx"
	case strings.Contains(sourceType, "application/vnd.ms-excel"):
		return ".xls"
	case strings.Contains(sourceType, "spreadsheetml.sheet"):
		return ".xlsx"
	case strings.Contains(sourceType, "application/vnd.ms-powerpoint"):
		return ".ppt"
	case strings.Contains(sourceType, "presentationml.presentation"):
		return ".pptx"
	case strings.Contains(sourceType, "application/rtf"), strings.Contains(sourceType, "text/rtf"):
		return ".rtf"
	default:
		return ".bin"
	}
}
