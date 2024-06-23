package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// MessageSender 结构体定义了发送消息所需的客户端和基础URL
type MessageSender struct {
	Client  *http.Client
	BaseURL string
}

// CreatMessageSender 构造函数初始化 MessageSender 实例
func CreatMessageSender(baseURL string) *MessageSender {
	return &MessageSender{
		Client:  &http.Client{}, // 使用默认 HTTP 客户端
		BaseURL: baseURL,
	}
}

// SendPrivateMessage 发送私聊消息
func (ms *MessageSender) SendPrivateMessage(userID int64, message string) (*http.Response, error) {
	return ms.sendMessage("send_private_msg", userID, message)
}

// SendGroupMessage 发送群聊消息
func (ms *MessageSender) SendGroupMessage(groupID int64, message string) (*http.Response, error) {
	return ms.sendMessage("send_group_msg", groupID, message)
}

// sendMessage 是一个私有方法，用于构建和发送消息请求
func (ms *MessageSender) sendMessage(endpoint string, targetID int64, message string) (*http.Response, error) {
	// 构造请求的完整 URL
	url := fmt.Sprintf("%s/%s", ms.BaseURL, endpoint)

	// 构造 JSON 数据负载
	data := map[string]interface{}{
		"message": message,
	}
	if endpoint == "send_private_msg" {
		data["user_id"] = targetID
	} else if endpoint == "send_group_msg" {
		data["group_id"] = targetID
		delete(data, "user_id")
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// 创建 POST 请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求并返回响应
	return ms.Client.Do(req)
}
