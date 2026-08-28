package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client 定义大语言模型文本补全/对话接口
type Client interface {
	Complete(ctx context.Context, prompt string, opts ...Option) (string, error)
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string, opts ...Option) (string, error)
}

// EmbeddingClient 定义向量嵌入接口
type EmbeddingClient interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// Options LLM 调用选项
type Options struct {
	Temperature float32
	MaxTokens   int
	JSONMode    bool
}

// Option 函数式选项
type Option func(*Options)

// WithTemperature 设置采样温度
func WithTemperature(t float32) Option {
	return func(o *Options) {
		o.Temperature = t
	}
}

// WithMaxTokens 设置最大生成 Token
func WithMaxTokens(m int) Option {
	return func(o *Options) {
		o.MaxTokens = m
	}
}

// WithJSONMode 设置是否强制 JSON 格式
func WithJSONMode(jsonMode bool) Option {
	return func(o *Options) {
		o.JSONMode = jsonMode
	}
}

// OpenAIClient 兼容 OpenAI 格式 API 的轻量实现 (支持 OpenAI, Azure, DeepSeek, Ollama, vLLM 等)
type OpenAIClient struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// NewOpenAIClient 创建 OpenAI 兼容客户端
func NewOpenAIClient(baseURL, apiKey, model string) *OpenAIClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float32         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete 简单 Prompt 补全
func (c *OpenAIClient) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt, opts...)
}

// CompleteWithSystem 带 System Prompt 的补全
func (c *OpenAIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string, opts ...Option) (string, error) {
	opt := Options{
		Temperature: 0.1,
		MaxTokens:   4096,
	}
	for _, o := range opts {
		o(&opt)
	}

	messages := make([]chatMessage, 0, 2)
	if systemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, chatMessage{Role: "user", Content: userPrompt})

	reqBody := chatRequest{
		Model:       c.Model,
		Messages:    messages,
		Temperature: opt.Temperature,
		MaxTokens:   opt.MaxTokens,
	}
	if opt.JSONMode {
		reqBody.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api error status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal chat response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("api returned error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type embedResponse struct {
	Data  []embedData `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Embed 单条文本向量化
func (c *OpenAIClient) Embed(ctx context.Context, text string) ([]float32, error) {
	res, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return res[0], nil
}

// EmbedBatch 批量文本向量化
func (c *OpenAIClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := embedRequest{
		Model: c.Model,
		Input: texts,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http embedding post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api embedding error status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var embedResp embedResponse
	if err := json.Unmarshal(bodyBytes, &embedResp); err != nil {
		return nil, fmt.Errorf("unmarshal embedding response: %w", err)
	}

	if embedResp.Error != nil {
		return nil, fmt.Errorf("api embedding error: %s", embedResp.Error.Message)
	}

	embeddings := make([][]float32, len(texts))
	for _, item := range embedResp.Data {
		if item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}
	return embeddings, nil
}
