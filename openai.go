package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

// Response represents the structure of the OpenAI chat completion response.
type Response struct {
	ID                string    `json:"id"`
	Object            string    `json:"object"`
	Created           int64     `json:"created"`
	Model             string    `json:"model"`
	SystemFingerprint string    `json:"system_fingerprint"`
	Choices           []Choice  `json:"choices"`
	Usage             UsageInfo `json:"usage"`
	Error             APIError  `json:"error"` // 增加错误处理字段
}

type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
}

// Choice represents the choice structure in the response.
type Choice struct {
	Index        int              `json:"index"`
	Message      Message          `json:"message"`
	LogProbs     *json.RawMessage `json:"logprobs"` // Use json.RawMessage for optional null handling
	FinishReason string           `json:"finish_reason"`
}

// Message represents the message structure in a choice.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Tokens  int    `json:"-"` // 记录这条消息使用的 token 数量
}

// UsageInfo represents the usage information of tokens.
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// userMessagesContext stores the history of messages for each user.
var userMessagesContext = make(map[int64][]Message)
var mutex sync.Mutex

// createProxyClient initializes and returns an http.Client with proxy settings.
func createProxyClient() (*http.Client, error) {
	// Consider simplifying your proxy setup based on your actual needs
	proxyURL, err := url.Parse("http://127.0.0.1:7890")
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	httpTransport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	return &http.Client{Transport: httpTransport}, nil
}

// 向 OpenAI 发送请求并打印回复
func queryOpenAI(message string) (*Response, error) {
	// 构建请求体
	requestData := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]string{
			{
				"role": "system",
				"content": "你是一个友好的QQ机器人，名字叫咕咕鼠，用来帮助QQ好友解决他们的问题。" +
					"你的创造者是鼠爹，他的QQ号是287816226",
			},
			{
				"role":    "user",
				"content": message,
			},
		},
	}
	requestBody, _ := json.Marshal(requestData)

	// 创建 HTTP 客户端和请求
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("creating request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))

	resp, err := openaiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request to OpenAI failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body failed: %w", err)
	}

	var response Response
	if err = json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %w", err)
	}

	return &response, nil
}

// 检查历史消息并更新
func updateHistoryWithResponse(userID int64, userMessage, assistantMessage Message) {
	mutex.Lock()
	defer mutex.Unlock()

	messages := userMessagesContext[userID]

	// 计算历史 token 总量，排除当前消息
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += msg.Tokens
	}

	// 添加新消息前检查历史消息的限制
	print("totalTokens: ", totalTokens, " len(messages): ", len(messages))
	if totalTokens > 10000 || len(messages) >= 10 {
		fmt.Println("Historical token limit or round limit exceeded, adjusting history.")
		// 如果超出，从最早的消息开始移除，直到满足条件
		for totalTokens > 15000 || len(messages) >= 10 {
			totalTokens -= messages[0].Tokens
			messages = messages[1:]
		}
	}

	// 添加用户问题和助手回答
	messages = append(messages, userMessage, assistantMessage)
	userMessagesContext[userID] = messages
}

func queryOpenAIWithContext(userID int64, userMessageContent string) (*Response, error) {
	// 检查消息内容是否以特定字符串开始，确定使用的模型
	model := "gpt-3.5-turbo" // 默认使用 GPT-3.5
	if strings.HasPrefix(userMessageContent, "/GPT4") {
		model = "gpt-4o"                            // 如果文本以 /GPT4 开头，改用 GPT-4o
		userMessageContent = userMessageContent[5:] // 移除前缀以发送实际的消息内容
		_, err := QQMessageSender.SendPrivateMessage(userID, "本次回答将使用 GPT-4o 模型，请稍等..")
		if err != nil {
			return nil, fmt.Errorf("error sending message: %w", err)
		}
	}
	// 准备请求数据
	requestData := map[string]interface{}{
		"model": model,
		"messages": append(userMessagesContext[userID], Message{
			Role:    "user",
			Content: userMessageContent,
		}), // 发送历史消息
	}
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request data: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))

	resp, err := openaiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request to OpenAI: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var response Response
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %w", err)
	}

	// 解析回答和 token 消耗
	if len(response.Choices) > 0 {
		userMessage := Message{
			Role:    "user",
			Content: userMessageContent,
			Tokens:  response.Usage.PromptTokens, // 使用提问的 token 消耗
		}
		assistantMessage := Message{
			Role:    "assistant",
			Content: response.Choices[0].Message.Content,
			Tokens:  response.Usage.CompletionTokens, // 使用回答的 token 消耗
		}
		updateHistoryWithResponse(userID, userMessage, assistantMessage)
	}
	return &response, nil
}

// 定义消息结构体
type ImageMessageContent struct {
	Type     string   `json:"type"`
	Text     string   `json:"text,omitempty"`
	ImageURL ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type ImageMessage struct {
	Role    string                `json:"role"`
	Content []ImageMessageContent `json:"content"`
}

type ImageRequestData struct {
	Model         string         `json:"model"`
	ImageMessages []ImageMessage `json:"messages"`
}

// 发送请求到 OpenAI
func queryOpenAIWithImage(userId int64, userMessageContent string, imageUrls []string) (*Response, error) {
	// 如果用户消息内容为空，则使用默认文本
	if userMessageContent == "" {
		userMessageContent = "请描述这张图片"
	}

	// 构建内容数组，包括文本和多张图片
	contents := []ImageMessageContent{
		{
			Type: "text",
			Text: userMessageContent,
		},
	}

	// 循环添加图片 URL 到内容数组
	for _, imageUrl := range imageUrls {
		contents = append(contents, ImageMessageContent{
			Type: "image_url",
			ImageURL: ImageURL{
				URL: imageUrl,
			},
		})
	}

	// 构建请求数据
	requestData := ImageRequestData{
		Model: "gpt-4o",
		ImageMessages: []ImageMessage{
			{
				Role:    "user",
				Content: contents,
			},
		},
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request data: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))

	// 发送请求并获取响应
	resp, err := openaiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request to OpenAI: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var response Response
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %w", err)
	}

	if len(response.Choices) > 0 {
		userMessage := Message{
			Role:    "user",
			Content: userMessageContent,
			Tokens:  response.Usage.PromptTokens,
		}
		assistantMessage := Message{
			Role:    "assistant",
			Content: response.Choices[0].Message.Content,
			Tokens:  response.Usage.CompletionTokens,
		}
		updateHistoryWithResponse(userId, userMessage, assistantMessage)
	}

	return &response, nil
}
