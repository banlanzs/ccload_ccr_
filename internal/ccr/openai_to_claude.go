package ccr

import (
	"fmt"

	"github.com/bytedance/sonic"
)

// OpenAIToClaude 实现 OpenAI 格式到 Claude 格式的转换
type OpenAIToClaude struct{}

func (t *OpenAIToClaude) Name() string {
	return "openai_to_claude"
}

// TransformRequest 转换 OpenAI 格式请求为 Claude 格式
func (t *OpenAIToClaude) TransformRequest(body []byte) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}

	var req map[string]any
	if err := sonic.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse request body failed: %w", err)
	}

	// 提取 system message 并转换为 Claude 的 system 字段
	if messages, ok := req["messages"].([]any); ok {
		systemMsg, userMessages := extractSystemMessage(messages)
		if systemMsg != "" {
			req["system"] = systemMsg
		}

		normalized, err := normalizeMessages(userMessages)
		if err != nil {
			return nil, fmt.Errorf("normalize messages failed: %w", err)
		}
		req["messages"] = normalized
	}

	// 处理 max_completion_tokens -> max_tokens 映射
	if maxCompletionTokens, ok := req["max_completion_tokens"]; ok {
		if _, hasMaxTokens := req["max_tokens"]; !hasMaxTokens {
			req["max_tokens"] = maxCompletionTokens
		}
		delete(req, "max_completion_tokens")
	}

	// 转换 tools 格式（OpenAI → Claude）
	if tools, ok := req["tools"].([]any); ok {
		claudeTools, err := convertToolsToClaudeFormat(tools)
		if err != nil {
			return nil, fmt.Errorf("convert tools failed: %w", err)
		}
		req["tools"] = claudeTools
	}

	// 转换 tool_choice 格式
	if toolChoice, ok := req["tool_choice"]; ok {
		claudeToolChoice := convertToolChoiceToClaudeFormat(toolChoice)
		if claudeToolChoice != nil {
			req["tool_choice"] = claudeToolChoice
		} else {
			delete(req, "tool_choice")
		}
	}

	// 移除不兼容的字段
	delete(req, "stream_options")
	delete(req, "response_format") // Claude 不支持 response_format

	out, err := sonic.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal transformed request failed: %w", err)
	}

	return out, nil
}

// extractSystemMessage 提取 system message 并返回剩余的 user/assistant messages
func extractSystemMessage(messages []any) (string, []any) {
	var systemContent string
	userMessages := make([]any, 0, len(messages))

	for _, msg := range messages {
		msgMap, ok := msg.(map[string]any)
		if !ok {
			userMessages = append(userMessages, msg)
			continue
		}

		role, _ := msgMap["role"].(string)
		if role == "system" {
			// 提取 system message 内容
			if content, ok := msgMap["content"].(string); ok {
				if systemContent != "" {
					systemContent += "\n\n" + content
				} else {
					systemContent = content
				}
			}
		} else {
			userMessages = append(userMessages, msg)
		}
	}

	return systemContent, userMessages
}

// normalizeMessages 规范化消息格式
func normalizeMessages(messages []any) ([]map[string]any, error) {
	result := make([]map[string]any, 0, len(messages))

	for i, msg := range messages {
		msgMap, ok := msg.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid message at index %d", i)
		}

		role, ok := msgMap["role"].(string)
		if !ok || role == "" {
			return nil, fmt.Errorf("missing role at index %d", i)
		}

		normalized := map[string]any{
			"role": role,
		}

		// 转换 content 字段
		content, err := normalizeContent(msgMap["content"])
		if err != nil {
			return nil, fmt.Errorf("normalize content at index %d: %w", i, err)
		}
		normalized["content"] = content

		// 处理 tool_calls（OpenAI → Claude tool_use）
		if toolCalls, ok := msgMap["tool_calls"].([]any); ok && len(toolCalls) > 0 {
			claudeContent, err := convertToolCallsToClaudeContent(toolCalls)
			if err != nil {
				return nil, fmt.Errorf("convert tool_calls at index %d: %w", i, err)
			}
			// Claude 使用 content 数组包含 tool_use 块
			normalized["content"] = claudeContent
		}

		// 处理 tool role（OpenAI → Claude）
		if role == "tool" {
			if toolCallID, ok := msgMap["tool_call_id"].(string); ok {
				// Claude 使用 tool_result 类型
				normalized["role"] = "user"
				normalized["content"] = []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": toolCallID,
						"content":     content,
					},
				}
			}
		}

		// 保留可选字段
		if name, ok := msgMap["name"]; ok {
			normalized["name"] = name
		}

		result = append(result, normalized)
	}

	return result, nil
}

// normalizeContent 规范化消息内容为 Claude 格式
func normalizeContent(raw any) (any, error) {
	switch v := raw.(type) {
	case string:
		// 简单文本消息
		return v, nil

	case []any:
		// 多部分消息（文本 + 图片等）
		parts := make([]map[string]any, 0, len(v))
		for i, part := range v {
			partMap, ok := part.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid content part at index %d", i)
			}

			partType, _ := partMap["type"].(string)
			switch partType {
			case "text":
				text, _ := partMap["text"].(string)
				parts = append(parts, map[string]any{
					"type": "text",
					"text": text,
				})
			case "image_url":
				// 保持图片 URL 格式（Claude API 也支持）
				parts = append(parts, partMap)
			default:
				// 其他类型直接透传
				parts = append(parts, partMap)
			}
		}
		return parts, nil

	case nil:
		return "", nil

	default:
		return nil, fmt.Errorf("unsupported content type: %T", raw)
	}
}

// convertToolCallsToClaudeContent 转换 OpenAI tool_calls 为 Claude content 数组
func convertToolCallsToClaudeContent(toolCalls []any) ([]map[string]any, error) {
	content := make([]map[string]any, 0, len(toolCalls))

	for i, tc := range toolCalls {
		tcMap, ok := tc.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid tool_call at index %d", i)
		}

		// OpenAI: {id, type: "function", function: {name, arguments}}
		// Claude: {type: "tool_use", id, name, input}
		id, _ := tcMap["id"].(string)
		funcMap, _ := tcMap["function"].(map[string]any)
		if funcMap == nil {
			return nil, fmt.Errorf("missing function in tool_call at index %d", i)
		}

		name, _ := funcMap["name"].(string)
		argsStr, _ := funcMap["arguments"].(string)

		// 解析 arguments JSON 字符串为对象
		var input map[string]any
		if argsStr != "" {
			if err := sonic.UnmarshalString(argsStr, &input); err != nil {
				return nil, fmt.Errorf("parse tool arguments at index %d: %w", i, err)
			}
		} else {
			input = make(map[string]any)
		}

		content = append(content, map[string]any{
			"type":  "tool_use",
			"id":    id,
			"name":  name,
			"input": input,
		})
	}

	return content, nil
}

// convertToolsToClaudeFormat 转换 OpenAI tools 为 Claude tools 格式
func convertToolsToClaudeFormat(tools []any) ([]map[string]any, error) {
	claudeTools := make([]map[string]any, 0, len(tools))

	for i, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid tool at index %d", i)
		}

		// OpenAI: {type: "function", function: {name, description, parameters}}
		// Claude: {name, description, input_schema}
		funcMap, ok := toolMap["function"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("missing function in tool at index %d", i)
		}

		name, _ := funcMap["name"].(string)
		description, _ := funcMap["description"].(string)
		parameters, _ := funcMap["parameters"].(map[string]any)

		claudeTool := map[string]any{
			"name":         name,
			"description":  description,
			"input_schema": parameters,
		}

		claudeTools = append(claudeTools, claudeTool)
	}

	return claudeTools, nil
}

// convertToolChoiceToClaudeFormat 转换 OpenAI tool_choice 为 Claude 格式
func convertToolChoiceToClaudeFormat(toolChoice any) any {
	switch v := toolChoice.(type) {
	case string:
		// "auto", "none", "required" → 直接映射
		if v == "auto" || v == "any" {
			return map[string]any{"type": "auto"}
		}
		if v == "required" {
			return map[string]any{"type": "any"}
		}
		return nil

	case map[string]any:
		// OpenAI: {type: "function", function: {name}}
		// Claude: {type: "tool", name}
		if funcMap, ok := v["function"].(map[string]any); ok {
			if name, ok := funcMap["name"].(string); ok {
				return map[string]any{
					"type": "tool",
					"name": name,
				}
			}
		}
		return nil

	default:
		return nil
	}
}
