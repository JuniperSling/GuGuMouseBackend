package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// QQMessage 定义 JSON 结构体
type QQMessage struct {
	PostType    string `json:"post_type"`
	MessageType string `json:"message_type"`
	Message     string `json:"message,omitempty"`
	UserID      int64  `json:"user_id,omitempty"`
	GroupID     int64  `json:"group_id,omitempty"`
}

var QQMessageSender *MessageSender
var openaiClient *http.Client

func main() {
	// init
	os.Setenv("OPENAI_API_KEY", "PLEASE FILL YOUR KEY HERE")
	// 检查key 是否合法Bearer sk-key
	if !strings.HasPrefix(os.Getenv("OPENAI_API_KEY"), "sk") {
		// 报错
		fmt.Println("Invalid OpenAI API key, key should start with sk-")
		return
	}
	QQMessageSender = CreatMessageSender("http://0.0.0.0:5700")
	openaiClient, _ = createProxyClient()
	// listening
	http.HandleFunc("/", handleMessage)
	fmt.Println("Server is listening on port 5701")
	err := http.ListenAndServe(":5701", nil)
	if err != nil {
		return
	}

}

func handleMessage(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		// 不正确的请求方法
		w.WriteHeader(http.StatusOK)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	defer r.Body.Close()

	var msg QQMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 根据消息类型处理消息
	switch msg.MessageType {
	case "private":
		if msg.UserID == 0 || msg.Message == "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		handlePrivateMessage(msg)
	case "group":
		if msg.GroupID == 0 || msg.UserID == 0 || msg.Message == "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		handleGroupMessage(msg)
	default:
		// 不处理其他类型的消息
		w.WriteHeader(http.StatusOK)
		return
	}

	// 返回 HTTP 状态 200
	w.WriteHeader(http.StatusOK)
}

func handlePrivateMessage(msg QQMessage) {
	go func() {
		privateChatMessage := parseCQCode(msg.Message)
		fmt.Printf("Private message from %d: %s\n, imageUrl:%v", msg.UserID, privateChatMessage.Text, privateChatMessage.ImageURLList)
		// 检查消息长度
		roughEstimateTokens := RoughEstimateTokens(privateChatMessage.Text)
		println("roughEstimateTokens:", roughEstimateTokens)
		if roughEstimateTokens > 3000 {
			_, err := QQMessageSender.SendPrivateMessage(msg.UserID, "输入太长了～ 请不要超过 2000 个字符")
			if err != nil {
				fmt.Printf("Error sending message: %s\n", err)
			}
			return
		}

		var answer *Response
		var err error
		// 无需响应
		if len(privateChatMessage.ImageURLList) == 0 && strings.Trim(privateChatMessage.Text, " ") == "" {
			return
		}
		// 纯文本请求
		if len(privateChatMessage.ImageURLList) == 0 {
			// 拦截“无语”用于网络检测
			if privateChatMessage.Text == "无语" {
				_, err := QQMessageSender.SendPrivateMessage(msg.UserID, "家人们，谁懂啊！")
				if err != nil {
					fmt.Printf("Error sending message: %s\n", err)
				}
				return
			}
			// 请求OpenAI
			answer, err = queryOpenAIWithContext(msg.UserID, privateChatMessage.Text)
			if err != nil {
				fmt.Printf("Error querying OpenAI: %s\n", err)
				return
			}
			if answer.Error.Message != "" {
				_, err := QQMessageSender.SendPrivateMessage(msg.UserID, answer.Error.Message)
				if err != nil {
					fmt.Printf("Error sending message: %s\n", err)
				}
				return
			}
		} else {
			QQMessageSender.SendPrivateMessage(msg.UserID, "已触发老鼠识图，请稍等～")
			// 图片请求
			answer, err = queryOpenAIWithImage(msg.UserID, privateChatMessage.Text, privateChatMessage.ImageURLList)
			if err != nil {
				fmt.Printf("Error querying OpenAI: %s\n", err)
				return
			}
			if answer.Error.Message != "" {
				_, err := QQMessageSender.SendPrivateMessage(msg.UserID, answer.Error.Message)
				if err != nil {
					fmt.Printf("Error sending message: %s\n", err)
				}
				return
			}
		}

		fmt.Printf("Response from OpenAI: %s, token usage:%v", answer.Choices[0].Message.Content, answer.Usage.TotalTokens)
		_, err = QQMessageSender.SendPrivateMessage(msg.UserID, answer.Choices[0].Message.Content)
	}()
}

func handleGroupMessage(msg QQMessage) {
	groupChatMessage := parseCQCode(msg.Message)
	fmt.Printf("Group message from %d in group %d: %s\n", msg.UserID, msg.GroupID, groupChatMessage)
	// todo implement me
}
