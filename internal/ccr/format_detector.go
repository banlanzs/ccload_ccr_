package ccr

import (
	"bytes"
	"encoding/json"
	"strings"
)

// DetectFormatFromPayload 从请求体检测 API 格式
// 返回 ProviderFormat (openai/anthropic/gemini) 或 "" (无法检测)
// 检测优先级：Anthropic (最强信号) > Gemini > OpenAI (最通用)
func DetectFormatFromPayload(payload []byte) ProviderFormat {
	// Anthropic 最强显式信号
	if bytes.Contains(payload, []byte(`"anthropic_version"`)) {
		return FormatAnthropic
	}

	// Gemini 特征：contents + parts
	if bytes.Contains(payload, []byte(`"contents"`)) || bytes.Contains(payload, []byte(`"systemInstruction"`)) {
		return FormatGemini
	}

	// OpenAI 回退信号：messages + model
	if bytes.Contains(payload, []byte(`"messages"`)) && bytes.Contains(payload, []byte(`"model"`)) {
		return FormatOpenAI
	}

	return ""
}

// InferFormatFromChannelType 从渠道类型推断目标格式
func InferFormatFromChannelType(channelType string) ProviderFormat {
	channelType = strings.ToLower(strings.TrimSpace(channelType))
	switch channelType {
	case "anthropic", "codex":
		return FormatAnthropic
	case "openai":
		return FormatOpenAI
	case "gemini":
		return FormatGemini
	default:
		return ""
	}
}

// NeedsConversion 判断是否需要格式转换
func NeedsConversion(body []byte, channelType string) bool {
	srcFormat := DetectFormatFromPayload(body)
	if srcFormat == "" {
		return false // 无法检测源格式，不转换
	}

	dstFormat := InferFormatFromChannelType(channelType)
	if dstFormat == "" {
		return false // 无法确定目标格式，不转换
	}

	return srcFormat != dstFormat
}

// DetectContentType 检测请求内容类型（text/tool_call/tool_result）
type ContentType int

const (
	ContentTypeText ContentType = iota
	ContentTypeToolCall
	ContentTypeToolResult
)

// DetectMessageContentType 检测消息内容类型（用于判断是否需要特殊处理）
func DetectMessageContentType(content interface{}) ContentType {
	switch v := content.(type) {
	case string:
		return ContentTypeText
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if t, ok := m["type"].(string); ok {
					switch t {
					case "text":
						return ContentTypeText
					case "tool_use", "tool_call":
						return ContentTypeToolCall
					case "tool_result":
						return ContentTypeToolResult
					}
				}
			}
		}
	}
	return ContentTypeText
}

// SafeJSONMarshal 安全地序列化 JSON（避免循环引用导致的 panic）
func SafeJSONMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// SafeJSONUnmarshal 安全地反序列化 JSON
func SafeJSONUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
