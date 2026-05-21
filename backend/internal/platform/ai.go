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
	var prompt string
	if itemType == "QUESTION" {
		prompt = `你是一个精通前端开发的专家。请根据我上传的题库内容，生成一个单文件 HTML 题库自测页面。请严格遵守以下设计、结构和交互规范：

### 1. 技术栈与基础设定
- 使用单文件 HTML，严禁使用外部复杂的 JS 框架（Vue 或 React）。
- 引入 CDN 版的 Tailwind CSS (<script src="https://cdn.tailwindcss.com"></script>)。
- 字符编码设为 UTF-8，页面支持移动端响应式（Viewport 配置）。

### 2. 视觉与 UI 设计风格
- 整体风格：走极简现代、高级学术/备考风。主色调使用深色系（slate-950 和 indigo-950）搭配轻量背景（slate-50）。
- 顶部 Header：采用渐变色背景（from-slate-900 to-indigo-950），文字圆角（rounded-2xl），右侧包含精美的纯文本导航按钮。
- 题目卡片：每道题独立为一个白底卡片（bg-white），带微弱阴影（shadow-sm）和边框，悬停时有平滑阴影加深效果（transition hover:shadow-md）。
- 选项按钮：宽屏平铺（w-full），左侧带有一个圆形的字母徽章（A/B/C/D）。

### 3. 核心交互逻辑 (原生 JavaScript)
- 用户点击某个选项按钮时，触发 checkAnswer(questionId, selectedOption, correctOption, explanation) 函数。
- 状态切换（动态修改 Tailwind 类名）：
  - 如果选对：当前按钮背景变绿（bg-emerald-50）、边框变绿（border-emerald-500）；徽章变成绿底白字。
  - 如果选错：当前按钮背景变红（bg-rose-50）、边框变红（border-rose-400）；徽章变成红底白字。
  - 其他未点击的选项保持原样，且允许用户多次点击其他选项进行尝试。
- 解析区域（Explanation）：默认隐藏（hidden）。点击选项后展示，选对时框体呈绿色调（bg-emerald-50），选错时呈红色调（bg-rose-50），并在内部用原生 JS 动态填入对应题目的解析文字。

### 4. 代码结构示例
请将我提供的题库题目，严格按照以下 HTML 骨架渲染到 <main> 标签内（必须包含 script 标签实现 checkAnswer 函数）：
<div class="bg-white rounded-2xl shadow-sm border border-slate-200 p-5 md:p-6 transition hover:shadow-md mb-6">
    <h3 class="text-base md:text-lg font-bold text-slate-900 mb-4">【题号】. 【题目内容】</h3>
    <div class="flex flex-col space-y-3">
        <button data-q="【题号】" data-opt="A" onclick="checkAnswer('【题号】', 'A', '【正确答案】', '【解析内容】')" class="w-full text-left p-4 rounded-xl border-2 border-slate-200 bg-white hover:bg-slate-50 hover:border-indigo-300 transition font-medium flex items-start gap-3 outline-none">
            <span class="flex-shrink-0 w-6 h-6 rounded-full bg-slate-100 text-slate-600 font-bold text-xs flex items-center justify-center border border-slate-300 group-hover:bg-indigo-100">A</span>
            <span class="text-slate-700 text-sm md:text-base">【选项A内容】</span>
        </button>
        <!-- 必须生成完整的 B、C、D 选项，结构与 A 完全相同 -->
        <button data-q="【题号】" data-opt="B" onclick="checkAnswer('【题号】', 'B', '【正确答案】', '【解析内容】')" class="w-full text-left p-4 rounded-xl border-2 border-slate-200 bg-white hover:bg-slate-50 hover:border-indigo-300 transition font-medium flex items-start gap-3 outline-none">
            <span class="flex-shrink-0 w-6 h-6 rounded-full bg-slate-100 text-slate-600 font-bold text-xs flex items-center justify-center border border-slate-300 group-hover:bg-indigo-100">B</span>
            <span class="text-slate-700 text-sm md:text-base">【选项B内容】</span>
        </button>
        <!-- 其它选项 C, D 结构同上 -->
    </div>
    <div id="exp-【题号】" class="mt-4 p-4 rounded-xl hidden border"></div>
</div>

### 5. 内容输出格式与强制约束
- 必须将我上传的题库文档中的**所有题目**全部转换并输出，一道都不能少！绝对不允许只提取前几道题，也绝对不允许输出“已展示前10题”这种省略文案。
- 请基于我给出的题库文件，直接输出完整的、可运行的 HTML 代码。`
	} else if itemType == "KNOWLEDGE" {
		prompt = `你是一位精通前端开发的 UI/UX 设计师。请根据我后续提供的知识点内容，生成一个单文件、自适应（Responsive）的 HTML 知识背诵卡片页面。

请严格遵守以下设计与技术规范：

1. 技术栈与框架：
   - 必须使用纯原生 HTML5 和 JavaScript。
   - 样式必须完全使用 Tailwind CSS CDN（引入地址： https://cdn.tailwindcss.com）。
   - 禁止使用任何第三方重型 JS 框架（Vue 或 React）。

2. 视觉与 UI 风格（现代科技/极简精致风）：
   - 背景色：使用优雅、低饱和度的浅色背景（slate-50）。
   - 头部（Header）：使用深色渐变（from-slate-900 to-indigo-950），搭配亮色文字，营造出高级的“期末救星/学霸工具”氛围。
   - 卡片设计：正面为纯白背景，带微弱投影（shadow-sm）、精致边框（border-slate-200）和圆角（rounded-2xl）；背面为深色主题（bg-slate-900），形成强烈的视觉反差。
   - 状态标签：善用小巧的 Badge 标签（bg-indigo-50 text-indigo-600）来标记“必背重点”或“核心释义”。

3. 核心交互逻辑（3D 翻转卡片）：
   - 使用 CSS 3D 转换（Perspective、preserve-3d、backface-visibility: hidden）实现原生、丝滑的卡片翻转效果。
   - 交互触发：点击卡片任意区域，调用原生 JavaScript 函数切换 CSS 类（.flipped），使卡片沿 Y 轴旋转 180 度。
   - 卡片背面内容如果较多，需限制最大高度（max-h-[180px]）并允许纵向滚动（overflow-y-auto），以防撑破卡片。

4. 页面结构布局：
   - 顶部有一个精致的 Header，包含项目标题、副标题，以及右侧的三个小按钮（基础刷题、考点背诵、扩展进阶）。
   - 中间为操作提示（“💡 技巧：点击卡片任意位置...”）。
   - 主体部分采用单列或网格布局，卡片高度固定（h-80）。
   - 底部包含一个简洁的居中 Footer。

5. 卡片骨架示例：
   必须严格按照以下结构输出每张翻转卡片，确保 Tailwind CSS 类名完整，以防排版错乱（特别是相对定位、绝对定位以及背面翻转相关的属性）：
   <div class="group relative w-full h-80 perspective-1000" onclick="this.querySelector('.card-inner').classList.toggle('[transform:rotateY(180deg)]')">
       <div class="card-inner relative w-full h-full transition-transform duration-500 [transform-style:preserve-3d]">
           <!-- 正面 -->
           <div class="absolute inset-0 w-full h-full backface-hidden bg-white border border-slate-200 rounded-2xl shadow-sm p-6 flex flex-col items-center justify-center text-center hover:shadow-md transition">
               <span class="inline-block mb-4 px-3 py-1 bg-indigo-50 text-indigo-600 text-xs font-bold rounded-full">必背重点</span>
               <h3 class="text-xl font-bold text-slate-900">【知识点名称，例如：群众路线的内涵？】</h3>
           </div>
           <!-- 背面 -->
           <div class="absolute inset-0 w-full h-full backface-hidden [transform:rotateY(180deg)] bg-slate-900 border border-slate-800 rounded-2xl shadow-lg p-6 flex flex-col justify-start overflow-y-auto text-left">
               <h4 class="text-lg font-bold text-indigo-300 mb-3 border-b border-slate-700 pb-2">核心释义</h4>
               <div class="text-slate-300 text-sm leading-relaxed space-y-2">
                   【在此处分段填入详细的知识点解析和背诵内容】
               </div>
           </div>
       </div>
   </div>

6. 内容输出格式与强制约束：
   - 必须将我上传文档中的**所有知识点**全部提取并转换输出，一个都不能少！绝对不允许擅自省略、截断，或只输出部分内容。
   - 请将所有 HTML、CSS（Tailwind）和 JS 逻辑整合在同一个代码块中输出。
   - 我会为你提供具体的知识点数据，请将它们完美地嵌入到上述卡片结构中。`
	} else if itemType == "EXTENDED" {
		prompt = `你是一个精通前端开发（Tailwind CSS）的 AI 备考专家。请根据我之前提供/上传的题库内容，生成一个单文件 HTML 网页。

该网页定位为“期末救星 · 深度扩展延伸版”，旨在针对原题库知识点进行复杂应用场景的扩充，深度检验掌握情况。

请严格按照以下规范进行设计和编写：

### 1. 视觉与框架设计 (Visual UI)
- 框架引入：使用 CDN 引入 Tailwind CSS（ https://cdn.tailwindcss.com），不写复杂的外部 CSS，保持代码单文件高内聚。
- 配色方案：采用冷色调（slate-50、indigo-950、purple-600 作为点缀色）。
- 整体布局：最大宽度 max-w-4xl，居中对齐，背景色使用浅灰（bg-slate-50）。
- 顶部 Header：采用深色渐变（from-slate-900 to-indigo-950），带有醒目的应用标签、主标题、副标题，并在右上角配置三个微调按钮（基础刷题、考点背诵、扩展进阶，当前页面为“扩展进阶”激活态）。

### 2. 内容编排规范 (Content Rules)
- 题目类型：针对我提供的题库核心考点，将其升级为高频高阶题。你可以自由发挥，生成各种类型的扩展题（例如分析题、综合题、应用题等）。
- 题目数量：请尽可能多地提炼核心考点并生成对应的扩展题，不要少于 10 道。
- 卡片组件：每道题独立为一个白底（bg-white）、圆角（rounded-2xl）、微阴影（shadow-sm border border-slate-200）的卡片，悬停时有微弱阴影加深动画。
- 卡片顶部：必须包含一个紫色背景的胶囊标签（“深度技术扩展”）。
- 选项设计：使用 4 个标准的 Button 作为 A/B/C/D 选项，宽幅占满（w-full），左侧带有一个灰底圆形的字母序号，文字靠左对齐，悬停时边框和背景有淡紫色过渡效果。

### 3. 原生 JS 交互逻辑 (JavaScript Logic)
- 严禁刷新：点击选项时，通过纯前端 JavaScript 动态修改当前卡片的样式，不触发页面刷新。
- 函数设计：编写一个 checkAnswer(questionId, selectedOption, correctOption, explanation) 函数。
- 状态切换：
  - 如果用户答对：被选中的按钮变为绿底绿框（bg-emerald-50 border-emerald-500），序号变为绿底白字。下方动态展示绿色边框的提示框，判定显示“💡 判定：恭喜你答对了！”，并附带“深度技术复盘：[详细的原理解析]”。
  - 如果用户答错：被选中的按钮变为红底红框（bg-rose-50 border-rose-400），序号变为红底白字。下方动态展示红色边框的提示框，判定显示“⚠️ 判定：回答有误...”，并附带“复习切入点：[详细的原理解析]”。
- 动态显示：解析区域（id="exp-xxx"）初始状态为 hidden，点击任意选项后移除 hidden 动态渲染解析内容。

### 4. 代码结构示例
请将提炼出的题目，严格按照以下 HTML 骨架渲染到 <main> 标签内（必须包含 script 标签实现 checkAnswer 函数）：
<div class="bg-white rounded-2xl shadow-sm border border-slate-200 p-5 md:p-6 transition hover:shadow-md mb-6">
    <div class="mb-3"><span class="inline-block px-3 py-1 bg-purple-100 text-purple-700 text-xs font-bold rounded-full">深度技术扩展</span></div>
    <h3 class="text-base md:text-lg font-bold text-slate-900 mb-4">【题号】. 【扩展题目内容】</h3>
    <div class="flex flex-col space-y-3">
        <button data-q="【题号】" data-opt="A" onclick="checkAnswer('【题号】', 'A', '【正确答案】', '【解析内容】')" class="w-full text-left p-4 rounded-xl border-2 border-slate-200 bg-white hover:bg-slate-50 hover:border-purple-300 transition font-medium flex items-start gap-3 outline-none">
            <span class="flex-shrink-0 w-6 h-6 rounded-full bg-slate-100 text-slate-600 font-bold text-xs flex items-center justify-center border border-slate-300 group-hover:bg-purple-100">A</span>
            <span class="text-slate-700 text-sm md:text-base">【选项A内容】</span>
        </button>
        <!-- 必须生成完整的 B、C、D 选项，结构与 A 完全相同 -->
        <button data-q="【题号】" data-opt="B" onclick="checkAnswer('【题号】', 'B', '【正确答案】', '【解析内容】')" class="w-full text-left p-4 rounded-xl border-2 border-slate-200 bg-white hover:bg-slate-50 hover:border-purple-300 transition font-medium flex items-start gap-3 outline-none">
            <span class="flex-shrink-0 w-6 h-6 rounded-full bg-slate-100 text-slate-600 font-bold text-xs flex items-center justify-center border border-slate-300 group-hover:bg-purple-100">B</span>
            <span class="text-slate-700 text-sm md:text-base">【选项B内容】</span>
        </button>
        <!-- 其它选项 C, D 结构同上 -->
    </div>
    <div id="exp-【题号】" class="mt-4 p-4 rounded-xl hidden border"></div>
</div>

### 5. 当前科目与任务
- 本次生成的科目为：请根据上传的文档内容自动推断。
- 请阅读我上传的题库，尽可能多地提炼核心考点，生成对应的扩展题（不要少于 10 道），直接输出完整的、可运行的 HTML 代码，不要写任何多余的解释。`
	} else {
		prompt = fmt.Sprintf("请根据以上文档生成 %s 类型的期末复习资料HTML代码。", itemType)
	}

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
