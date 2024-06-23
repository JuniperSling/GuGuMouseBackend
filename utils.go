package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode"
)

// ChatMessage holds data extracted from a chat message including CQ codes.
type ChatMessage struct {
	Text         string   // Clean message text without CQ codes
	ImageURLList []string // URLs of images mentioned in the message
	AtUserList   []string // IDs of users that are mentioned
	ReplyTo      string   // ID of the message being replied to
}

var (
	// Precompiled regular expressions for parsing
	reCQ    = regexp.MustCompile(`\[CQ:[^]]+]`)
	reImage = regexp.MustCompile(`\[CQ:image,[^]]*url=([^,\]]+)]`)
	reAt    = regexp.MustCompile(`\[CQ:at,qq=(\d+)]`)
	reReply = regexp.MustCompile(`\[CQ:reply,id=(-?\d+)]`)
)

// parseCQCode parses a message string containing CQ codes and extracts details.
func parseCQCode(message string) ChatMessage {
	message = unescapeCQCode(message)
	cleanMessage := reCQ.ReplaceAllString(message, "")

	imageMatches := reImage.FindAllStringSubmatch(message, -1)
	var imageURLs []string
	for _, match := range imageMatches {
		if len(match) > 1 {
			imageURLs = append(imageURLs, match[1])
		}
	}

	atMatches := reAt.FindAllStringSubmatch(message, -1)
	var atUsers []string
	for _, match := range atMatches {
		if len(match) > 1 {
			atUsers = append(atUsers, match[1])
		}
	}

	replyMatches := reReply.FindAllStringSubmatch(message, -1)
	var replyTo string
	if len(replyMatches) > 0 && len(replyMatches[0]) > 1 {
		replyTo = replyMatches[0][1]
	}
	cleanMessage = strings.TrimSpace(cleanMessage)

	return ChatMessage{
		Text:         cleanMessage,
		ImageURLList: imageURLs,
		AtUserList:   atUsers,
		ReplyTo:      replyTo,
	}
}

// unescapeCQCode converts HTML entity sequences in a CQ code back to their respective characters.
func unescapeCQCode(message string) string {
	return html.UnescapeString(message)
}

// String formats the ChatMessage as a string in key-value pair format for logging.
func (c ChatMessage) String() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Text: %q, ", c.Text))
	builder.WriteString(fmt.Sprintf("ImageURLs: [%s], ", strings.Join(c.ImageURLList, ", ")))
	builder.WriteString(fmt.Sprintf("AtUsers: [%s], ", strings.Join(c.AtUserList, ", ")))
	builder.WriteString(fmt.Sprintf("ReplyTo: %q", c.ReplyTo))

	return builder.String()
}

// roughEstimateTokens 估计给定字符串的 token 数量。
func RoughEstimateTokens(input string) float64 {
	var tokenCount float64
	var inWord bool

	for _, runeValue := range input {
		switch {
		case unicode.Is(unicode.Scripts["Han"], runeValue):
			// 对于汉字，每个汉字计为 1.5 tokens
			tokenCount += 1.5
			inWord = false // 结束当前英文单词
		case runeValue == ' ' || unicode.IsPunct(runeValue):
			// 空格和标点符号不计入 tokens，但标记结束单词
			inWord = false
		default:
			// 对于连续的英文字母，只在词的开始处计算一个 token
			if !inWord {
				tokenCount++
				inWord = true // 开始一个新单词
			}
		}
	}

	return tokenCount
}
