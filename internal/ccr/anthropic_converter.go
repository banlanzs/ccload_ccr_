package ccr

import (
	"fmt"

	"github.com/bytedance/sonic"
)

type AnthropicConverter struct{}

func NewAnthropicConverter() *AnthropicConverter { return &AnthropicConverter{} }

type anthropicRequest struct {
	Model       string             `json:"model,omitempty"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	ToolChoice  any                `json:"tool_choice,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
	TopP        *float64           `json:"top_p,omitempty"`
	MaxTokens   *int               `json:"max_tokens,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

func (c *AnthropicConverter) ToCanonical(payload []byte) (*CanonicalRequest, error) {
	var req anthropicRequest
	if err := sonic.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("anthropic unmarshal: %w", err)
	}
	out := &CanonicalRequest{
		Model:       req.Model,
		System:      req.System,
		ToolChoice:  req.ToolChoice,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}
	for _, t := range req.Tools {
		out.Tools = append(out.Tools, CanonicalTool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}
	for _, m := range req.Messages {
		msg := CanonicalMessage{Role: m.Role}
		switch content := m.Content.(type) {
		case string:
			msg.Content = append(msg.Content, CanonicalPart{Type: "text", Text: content})
		case []interface{}:
			for _, p := range content {
				pm, ok := p.(map[string]interface{})
				if !ok {
					continue
				}
				pt, _ := pm["type"].(string)
				switch pt {
				case "text":
					txt, _ := pm["text"].(string)
					msg.Content = append(msg.Content, CanonicalPart{Type: "text", Text: txt})
				case "tool_use":
					id, _ := pm["id"].(string)
					name, _ := pm["name"].(string)
					input, _ := pm["input"].(map[string]interface{})
					msg.Content = append(msg.Content, CanonicalPart{
						Type: "tool_call",
						ToolCall: &CanonicalToolCall{
							ID:   id,
							Name: name,
							Args: input,
						},
					})
				case "tool_result":
					toolUseID, _ := pm["tool_use_id"].(string)
					txt, _ := pm["content"].(string)
					msg.Content = append(msg.Content, CanonicalPart{
						Type: "tool_result",
						ToolCall: &CanonicalToolCall{
							ID: toolUseID,
						},
						Text: txt,
					})
				}
			}
		}
		out.Messages = append(out.Messages, msg)
	}
	return out, nil
}

func (c *AnthropicConverter) FromCanonical(req *CanonicalRequest) ([]byte, error) {
	out := anthropicRequest{
		Model:       req.Model,
		System:      req.System,
		ToolChoice:  req.ToolChoice,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}
	for _, t := range req.Tools {
		out.Tools = append(out.Tools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}
	for _, m := range req.Messages {
		am := anthropicMessage{Role: m.Role}
		parts := make([]map[string]interface{}, 0, len(m.Content))
		for _, p := range m.Content {
			switch p.Type {
			case "text":
				parts = append(parts, map[string]interface{}{
					"type": "text",
					"text": p.Text,
				})
			case "tool_call":
				if p.ToolCall == nil {
					continue
				}
				parts = append(parts, map[string]interface{}{
					"type":  "tool_use",
					"id":    p.ToolCall.ID,
					"name":  p.ToolCall.Name,
					"input": p.ToolCall.Args,
				})
			case "tool_result":
				toolUseID := ""
				if p.ToolCall != nil {
					toolUseID = p.ToolCall.ID
				}
				parts = append(parts, map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": toolUseID,
					"content":     p.Text,
				})
			}
		}
		if len(parts) == 1 && parts[0]["type"] == "text" {
			am.Content = parts[0]["text"]
		} else {
			am.Content = parts
		}
		out.Messages = append(out.Messages, am)
	}
	b, err := sonic.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("anthropic marshal: %w", err)
	}
	return b, nil
}
