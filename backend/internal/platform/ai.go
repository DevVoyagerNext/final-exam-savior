package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"final-exam-savior/backend/internal/config"
)

// AIClient 定义了与大语言模型交互的核心接口
type AIClient interface {
	// GenerateHTML 基于提取出的纯文本生成 HTML 格式的期末复习资料
	GenerateHTML(ctx context.Context, itemType string, sourceText string) (string, error)

	// GenerateHTMLWithDocument 基于已上传到大模型平台（如阿里百炼）的文档ID生成 HTML 格式复习资料
	// fileID: 通过 UploadFile 接口获取的系统文件标识（例如: file-xxxx）
	GenerateHTMLWithDocument(ctx context.Context, itemType string, fileID string) (string, error)

	// OCRText 将图片 URL 传给视觉模型（VLM）提取纯文本
	OCRText(ctx context.Context, imageURL string) (string, error)

	// UploadFile 将物理文件流上传到大模型平台（如阿里百炼），返回一个可用于对话的 file_id
	UploadFile(ctx context.Context, fileName string, reader io.Reader) (string, error)
}

// OpenAICompatClient 实现了与 OpenAI API 格式兼容的大模型客户端
type OpenAICompatClient struct {
	cfg    config.AIConfig
	client *http.Client
}

type aiRequestError struct {
	StatusCode int
	Body       string
}

func (e *aiRequestError) Error() string {
	return fmt.Sprintf("ai api status %d: %s", e.StatusCode, e.Body)
}

func NewOpenAICompatClient(cfg config.AIConfig) *OpenAICompatClient {
	return &OpenAICompatClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// GenerateHTML 传统的文本输入模式：将长文本直接拼接到 prompt 中，让模型生成内容
func (c *OpenAICompatClient) GenerateHTML(ctx context.Context, itemType string, sourceText string) (string, error) {
	prompt := fmt.Sprintf("你是期末复习助手。请基于以下学习材料生成一份可离线打开的完整 HTML 页面。目标类型：%s。要求输出完整 html 文档，仅输出 HTML，不要额外解释。\n\n学习材料：\n%s", itemType, sourceText)
	return c.chatWithFallback(ctx, c.cfg.Model, []map[string]any{
		{
			"role":    "user",
			"content": prompt,
		},
	})
}

// GenerateHTMLWithDocument 阿里云百炼专属的高级文档模式：
// 不再手动提取文本，而是直接传入 file_id，模型会自动读取并理解原始文档（PDF/Word等）的内容和排版。
// 支持传入多个文件，例如多个 fileID 用逗号拼接："fileid://file-1,fileid://file-2"
func (c *OpenAICompatClient) GenerateHTMLWithDocument(ctx context.Context, itemType string, fileID string) (string, error) {
	prompt := fmt.Sprintf("请根据以上文档生成 %s 类型的期末复习资料HTML代码。", itemType)
	return c.chatWithFallback(ctx, c.cfg.Model, []map[string]any{
		// 阿里百炼特有的系统级文档引用语法：fileid://<你的file_id>
		{"role": "system", "content": fmt.Sprintf("fileid://%s", fileID)},
		{"role": "user", "content": prompt},
	})
}

// OCRText 调用视觉多模态大模型（如 Qwen-VL），通过传入图片 URL 实现高精度的 OCR 文本提取
func (c *OpenAICompatClient) OCRText(ctx context.Context, imageURL string) (string, error) {
	model := c.cfg.OCRModel
	if model == "" {
		model = c.cfg.Model
	}
	return c.chatWithFallback(ctx, model, []map[string]any{
		{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": "请对图片执行 OCR，输出尽量完整的纯文本，不要附加说明。"},
				{"type": "image_url", "image_url": map[string]string{"url": imageURL}},
			},
		},
	})
}

func (c *OpenAICompatClient) chatWithFallback(ctx context.Context, model string, messages []map[string]any) (string, error) {
	text, err := c.chat(ctx, model, messages)
	if err == nil {
		return text, nil
	}

	var requestErr *aiRequestError
	if !errors.As(err, &requestErr) || !isModelNotFoundError(requestErr) {
		return "", err
	}

	fallbackModel, fallbackErr := c.pickFallbackModel(ctx, model)
	if fallbackErr != nil {
		return "", fmt.Errorf("%w; auto model fallback failed: %v", err, fallbackErr)
	}
	if fallbackModel == "" || fallbackModel == model {
		return "", err
	}

	return c.chat(ctx, fallbackModel, messages)
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

	endpoint := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build ai request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.cfg.APIKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call ai api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", &aiRequestError{StatusCode: resp.StatusCode, Body: string(data)}
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

// UploadFile 阿里云百炼特有的文件上传接口：
// 将本地或对象存储中的物理文件（PDF/Word等）直接上传给大模型平台，
// 上传成功后会返回一个唯一的 file_id，该 file_id 随后可以在 GenerateHTMLWithDocument 接口中供大模型引用。
func (c *OpenAICompatClient) UploadFile(ctx context.Context, fileName string, reader io.Reader) (string, error) {
	if c.cfg.BaseURL == "" || c.cfg.APIKey == "" {
		return "", fmt.Errorf("openai compatible config is incomplete")
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		var copyErr error
		defer func() {
			writer.Close()
			if copyErr != nil {
				pw.CloseWithError(copyErr)
			} else {
				pw.Close()
			}
		}()

		// 阿里百炼提取文档内容的用途是 "file-extract"
		if err := writer.WriteField("purpose", "file-extract"); err != nil {
			copyErr = fmt.Errorf("write purpose field: %w", err)
			return
		}

		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			copyErr = fmt.Errorf("create form file: %w", err)
			return
		}
		if _, err := io.Copy(part, reader); err != nil {
			copyErr = fmt.Errorf("copy file content: %w", err)
			return
		}
	}()

	endpoint := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/") + "/files"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, pr)
	if err != nil {
		return "", fmt.Errorf("build upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.cfg.APIKey))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 为上传请求单独配置一个没有代理的 Transport，或者使用不带代理的 client
	uploadClient := &http.Client{
		Timeout:   c.client.Timeout,
		Transport: &http.Transport{Proxy: nil}, // 禁用代理
	}

	resp, err := uploadClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call upload api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("upload api status %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("upload response returned empty id")
	}
	return result.ID, nil
}

func (c *OpenAICompatClient) pickFallbackModel(ctx context.Context, currentModel string) (string, error) {
	models, err := c.listModels(ctx)
	if err != nil {
		return "", err
	}
	for _, candidate := range []string{
		"deepseek-chat",
		"deepseek-v3",
		"gpt-4.1-mini",
		"gpt-4o-mini",
		"gpt-4o",
		"glm-4-flash",
		"qwen-plus",
	} {
		if containsModel(models, candidate) && candidate != currentModel {
			return candidate, nil
		}
	}
	for _, model := range models {
		if model != currentModel {
			return model, nil
		}
	}
	return "", fmt.Errorf("no fallback model available")
}

func (c *OpenAICompatClient) listModels(ctx context.Context) ([]string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.cfg.APIKey))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call models api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("models api status %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}
	models := make([]string, 0, len(result.Data))
	for _, item := range result.Data {
		if strings.TrimSpace(item.ID) != "" {
			models = append(models, item.ID)
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("models response is empty")
	}
	return models, nil
}

func isModelNotFoundError(err *aiRequestError) bool {
	if err == nil {
		return false
	}
	body := strings.ToLower(err.Body)
	return err.StatusCode == http.StatusBadRequest &&
		(strings.Contains(body, "model does not exist") || strings.Contains(body, `"code":20012`))
}

func containsModel(models []string, target string) bool {
	for _, model := range models {
		if model == target {
			return true
		}
	}
	return false
}
