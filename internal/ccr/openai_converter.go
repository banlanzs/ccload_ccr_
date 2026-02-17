package ccr

import (
	"fmt"

	"github.com/bytedance/sonic"
)

type OpenAIConverter struct{}

func NewOpenAIConverter() *OpenAIConverter { return &OpenAIConverter{} }

type openAIRequest struct {
	Model       string          `json:"model,omitempty"`
	Messages    []openAIMessage `json:"messages,omitempty"`
	Tools       []openAITool    `json:"tools,omitempty"`
	ToolChoice  any             `json:"tool_choice,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content,omitempty"`
	Name       string           `json:"name,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAITool struct {
	Type     string         `json:"type,omitempty"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function openAICallFunction `json:"function"`
}

type openAICallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

func (c *OpenAIConverter) ToCanonical(payload []byte) (*CanonicalRequest, error) {
	var req openAIRequest
	if err := sonic.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("openai unmarshal: %w", err)
	}
	out := &CanonicalRequest{
		Model:       req.Model,
		ToolChoice:  req.ToolChoice,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}
	for _, t := range req.Tools {
		out.Tools = append(out.Tools, CanonicalTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		})
	}
	for _, m := range req.Messages {
		msg := CanonicalMessage{
			Role: m.Role,
			Name: m.Name,
		}
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
				case "image_url":
					msg.Content = append(msg.Content, CanonicalPart{Type: "image"})
				}
			}
		}
		for _, tc := range m.ToolCalls {
			args := map[string]interface{}{}
			if tc.Function.Arguments != "" {
				if err := sonic.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return nil, fmt.Errorf("openai tool args decode failed (name=%s): %w", tc.Function.Name, err)
				}
			}
			msg.Content = append(msg.Content, CanonicalPart{
				Type: "tool_call",
				ToolCall: &CanonicalToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
					Args: args,
				},
			})
		}
		out.Messages = append(out.Messages, msg)
	}
	return out, nil
}

func (c *OpenAIConverter) FromCanonical(req *CanonicalRequest) ([]byte, error) {
	out := openAIRequest{
		Model:       req.Model,
		ToolChoice:  req.ToolChoice,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}
	for _, t := range req.Tools {
		out.Tools = append(out.Tools, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	for _, m := range req.Messages {
		om := openAIMessage{
			Role: m.Role,
			Name: m.Name,
		}
		textParts := make([]map[string]interface{}, 0)
		for _, p := range m.Content {
			switch p.Type {
			case "text":
				textParts = append(textParts, map[string]interface{}{
					"type": "text",
					"text": p.Text,
				})
			case "tool_call":
				if p.ToolCall == nil {
					continue
				}
				argBytes, _ := sonic.Marshal(p.ToolCall.Args)
				om.ToolCalls = append(om.ToolCalls, openAIToolCall{
					ID:   p.ToolCall.ID,
					Type: "function",
					Function: openAICallFunction{
						Name:      p.ToolCall.Name,
						Arguments: string(argBytes),
					},
				})
			}
		}
		if len(textParts) == 1 {
			if text, ok := textParts[0]["text"].(string); ok {
				om.Content = text
			}
		} else if len(textParts) > 1 {
			om.Content = textParts
		} else if len(om.ToolCalls) > 0 {
			// Keep explicit null content for tool-call-only assistant messages.
			om.Content = nil
		}
		out.Messages = append(out.Messages, om)
	}
	b, err := sonic.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("openai marshal: %w", err)
	}
	return b, nil
}
