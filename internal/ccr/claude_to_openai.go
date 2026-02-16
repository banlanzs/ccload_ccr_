package ccr

import (
	"fmt"

	"github.com/bytedance/sonic"
)

// ClaudeToOpenAI 实现 Claude 格式到 OpenAI 格式的响应转换
type ClaudeToOpenAI struct{}

func (t *ClaudeToOpenAI) Name() string {
	return "claude_to_openai"
}

// TransformResponse 转换 Claude 格式响应为 OpenAI 格式（非流式）
func (t *ClaudeToOpenAI) TransformResponse(body []byte) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}

	var claudeResp map[string]any
	if err := sonic.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("parse claude response failed: %w", err)
	}

	// 检查是否是错误响应
	if errObj, ok := claudeResp["error"]; ok {
		// 转换错误格式
		return t.transformErrorResponse(errObj)
	}

	// 获取响应类型
	respType, _ := claudeResp["type"].(string)

	switch respType {
	case "message":
		return t.transformMessageResponse(claudeResp)
	case "message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop":
		// 流式事件应该用 TransformStreamEvent
		return body, nil
	default:
		// 未知类型，原样返回
		return body, nil
	}
}

// transformMessageResponse 转换 Claude message 响应为 OpenAI 格式
func (t *ClaudeToOpenAI) transformMessageResponse(claudeResp map[string]any) ([]byte, error) {
	// Claude: {
	//   id, type: "message", role: "assistant", content: [...],
	//   model, stop_reason, usage: {input_tokens, output_tokens}
	// }
	// OpenAI: {
	//   id, object: "chat.completion", choices: [{...}],
	//   model, usage: {prompt_tokens, completion_tokens, total_tokens}
	// }

	id, _ := claudeResp["id"].(string)
	model, _ := claudeResp["model"].(string)

	// 提取内容
	content := t.extractTextContent(claudeResp["content"])
	toolCalls := t.extractToolCalls(claudeResp["content"])
	stopReason, _ := claudeResp["stop_reason"].(string)

	// 构建 choice
	choice := map[string]any{
		"index": 0,
		"message": map[string]any{
			"role":    "assistant",
			"content": content,
		},
		"finish_reason": t.convertFinishReason(stopReason),
	}

	// 如果有 tool_calls，添加到 message
	if len(toolCalls) > 0 {
		choice["message"].(map[string]any)["tool_calls"] = toolCalls
	}

	// 提取 usage
	usage, _ := claudeResp["usage"].(map[string]any)
	openAIUsage := t.convertUsage(usage)

	// 构建 OpenAI 响应
	openAIResp := map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": 0, // Claude 不提供 created，设为 0
		"model":   model,
		"choices": []any{choice},
		"usage":   openAIUsage,
	}

	return sonic.Marshal(openAIResp)
}

// transformErrorResponse 转换错误响应
func (t *ClaudeToOpenAI) transformErrorResponse(errObj any) ([]byte, error) {
	errMap, ok := errObj.(map[string]any)
	if !ok {
		return sonic.Marshal(map[string]any{
			"error": map[string]any{
				"message": fmt.Sprintf("%v", errObj),
				"type":    "invalid_request_error",
			},
		})
	}

	// Claude error: {type, message}
	// OpenAI error: {error: {message, type, code}}
	message, _ := errMap["message"].(string)
	errType, _ := errMap["type"].(string)

	return sonic.Marshal(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errType,
			"code":    errType,
		},
	})
}

// TransformStreamEvent 转换 Claude 流式事件为 OpenAI 格式
func (t *ClaudeToOpenAI) TransformStreamEvent(event []byte) ([]byte, error) {
	if len(event) == 0 {
		return event, nil
	}

	// 解析 SSE data 部分
	var claudeEvent map[string]any
	if err := sonic.Unmarshal(event, &claudeEvent); err != nil {
		return nil, fmt.Errorf("parse stream event failed: %w", err)
	}

	eventType, _ := claudeEvent["type"].(string)

	switch eventType {
	case "message_start":
		return t.transformMessageStart(claudeEvent)
	case "content_block_start":
		return t.transformContentBlockStart(claudeEvent)
	case "content_block_delta":
		return t.transformContentBlockDelta(claudeEvent)
	case "content_block_stop":
		return t.transformContentBlockStop(claudeEvent)
	case "message_delta":
		return t.transformMessageDelta(claudeEvent)
	case "message_stop":
		return t.transformMessageStop()
	case "ping":
		// 忽略 ping 事件
		return nil, nil
	case "error":
		return t.transformStreamError(claudeEvent)
	default:
		// 未知事件类型，原样返回
		return event, nil
	}
}

func (t *ClaudeToOpenAI) transformMessageStart(event map[string]any) ([]byte, error) {
	message, _ := event["message"].(map[string]any)
	if message == nil {
		return nil, nil
	}

	id, _ := message["id"].(string)
	model, _ := message["model"].(string)

	openAIChunk := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   model,
		"choices": []any{
			map[string]any{
				"index": 0,
				"delta": map[string]any{
					"role":    "assistant",
					"content": "",
				},
				"finish_reason": nil,
			},
		},
	}

	return sonic.Marshal(openAIChunk)
}

func (t *ClaudeToOpenAI) transformContentBlockStart(event map[string]any) ([]byte, error) {
	index, _ := event["index"].(float64)
	contentBlock, _ := event["content_block"].(map[string]any)
	if contentBlock == nil {
		return nil, nil
	}

	blockType, _ := contentBlock["type"].(string)

	// 处理 tool_use 开始
	if blockType == "tool_use" {
		id, _ := contentBlock["id"].(string)
		name, _ := contentBlock["name"].(string)

		openAIChunk := map[string]any{
			"id":      "",
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   "",
			"choices": []any{
				map[string]any{
					"index": int(index),
					"delta": map[string]any{
						"tool_calls": []any{
							map[string]any{
								"index": int(index),
								"id":    id,
								"type":  "function",
								"function": map[string]any{
									"name":      name,
									"arguments": "",
								},
							},
						},
					},
					"finish_reason": nil,
				},
			},
		}
		return sonic.Marshal(openAIChunk)
	}

	// text 类型不发送单独的 content_block_start
	return nil, nil
}

func (t *ClaudeToOpenAI) transformContentBlockDelta(event map[string]any) ([]byte, error) {
	index, _ := event["index"].(float64)
	delta, _ := event["delta"].(map[string]any)
	if delta == nil {
		return nil, nil
	}

	deltaType, _ := delta["type"].(string)

	var openAIDelta map[string]any

	switch deltaType {
	case "text_delta":
		text, _ := delta["text"].(string)
		openAIDelta = map[string]any{
			"content": text,
		}

	case "input_json_delta":
		// tool_use 的参数增量
		partialJSON, _ := delta["partial_json"].(string)
		openAIDelta = map[string]any{
			"tool_calls": []any{
				map[string]any{
					"index": int(index),
					"function": map[string]any{
						"arguments": partialJSON,
					},
				},
			},
		}

	default:
		return nil, nil
	}

	openAIChunk := map[string]any{
		"id":      "",
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   "",
		"choices": []any{
			map[string]any{
				"index":          int(index),
				"delta":          openAIDelta,
				"finish_reason": nil,
			},
		},
	}

	return sonic.Marshal(openAIChunk)
}

func (t *ClaudeToOpenAI) transformContentBlockStop(event map[string]any) ([]byte, error) {
	// content_block_stop 不需要发送单独的 chunk
	return nil, nil
}

func (t *ClaudeToOpenAI) transformMessageDelta(event map[string]any) ([]byte, error) {
	delta, _ := event["delta"].(map[string]any)
	usage, _ := event["usage"].(map[string]any)

	var finishReason any
	if delta != nil {
		stopReason, _ := delta["stop_reason"].(string)
		finishReason = t.convertFinishReason(stopReason)
	}

	openAIChunk := map[string]any{
		"id":      "",
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   "",
		"choices": []any{
			map[string]any{
				"index":          0,
				"delta":          map[string]any{},
				"finish_reason":  finishReason,
			},
		},
	}

	// 添加 usage（如果有）
	if usage != nil {
		openAIChunk["usage"] = t.convertStreamUsage(usage)
	}

	return sonic.Marshal(openAIChunk)
}

func (t *ClaudeToOpenAI) transformMessageStop() ([]byte, error) {
	// OpenAI 使用 [DONE] 标记流结束
	return []byte("[DONE]"), nil
}

func (t *ClaudeToOpenAI) transformStreamError(event map[string]any) ([]byte, error) {
	errObj, _ := event["error"].(map[string]any)
	if errObj == nil {
		return nil, nil
	}

	message, _ := errObj["message"].(string)
	errType, _ := errObj["type"].(string)

	openAIError := map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errType,
			"code":    errType,
		},
	}

	return sonic.Marshal(openAIError)
}

// 辅助函数

func (t *ClaudeToOpenAI) extractTextContent(content any) string {
	blocks, ok := content.([]any)
	if !ok {
		return ""
	}

	var text string
	for _, block := range blocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}
		blockType, _ := blockMap["type"].(string)
		if blockType == "text" {
			if t, ok := blockMap["text"].(string); ok {
				text += t
			}
		}
	}

	return text
}

func (t *ClaudeToOpenAI) extractToolCalls(content any) []any {
	blocks, ok := content.([]any)
	if !ok {
		return nil
	}

	var toolCalls []any
	for _, block := range blocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}
		blockType, _ := blockMap["type"].(string)
		if blockType == "tool_use" {
			id, _ := blockMap["id"].(string)
			name, _ := blockMap["name"].(string)
			input := blockMap["input"]

			// 将 input 转换为 JSON 字符串
			argsJSON, _ := sonic.MarshalString(input)

			toolCalls = append(toolCalls, map[string]any{
				"id":   id,
				"type": "function",
				"function": map[string]any{
					"name":      name,
					"arguments": string(argsJSON),
				},
			})
		}
	}

	return toolCalls
}

func (t *ClaudeToOpenAI) convertFinishReason(stopReason string) any {
	switch stopReason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	default:
		if stopReason != "" {
			return "stop"
		}
		return nil
	}
}

func (t *ClaudeToOpenAI) convertUsage(usage map[string]any) map[string]any {
	if usage == nil {
		return nil
	}

	inputTokens, _ := usage["input_tokens"].(float64)
	outputTokens, _ := usage["output_tokens"].(float64)

	return map[string]any{
		"prompt_tokens":     int(inputTokens),
		"completion_tokens": int(outputTokens),
		"total_tokens":      int(inputTokens + outputTokens),
	}
}

func (t *ClaudeToOpenAI) convertStreamUsage(usage map[string]any) map[string]any {
	if usage == nil {
		return nil
	}

	// Claude 流式 usage 可能有 output_tokens
	outputTokens, _ := usage["output_tokens"].(float64)

	return map[string]any{
		"completion_tokens": int(outputTokens),
		"total_tokens":      int(outputTokens),
	}
}
