package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"final-exam-savior/backend/internal/config"
)

type AIClient interface {
	GenerateHTML(ctx context.Context, itemType string, sourceText string) (string, error)
	OCRText(ctx context.Context, imageURL string) (string, error)
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
