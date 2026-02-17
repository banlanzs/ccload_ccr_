package ccr

import (
	"fmt"
	"strconv"

	"github.com/bytedance/sonic"
)

type GeminiConverter struct{}

func NewGeminiConverter() *GeminiConverter { return &GeminiConverter{} }

type geminiRequest struct {
	SystemInstruction *geminiContent   `json:"system_instruction,omitempty"`
	Contents          []geminiContent  `json:"contents,omitempty"`
	Tools             []geminiToolWrap `json:"tools,omitempty"`
	GenerationConfig  map[string]any   `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts,omitempty"`
}

type geminiPart struct {
	Text         string                 `json:"text,omitempty"`
	InlineData   map[string]interface{} `json:"inlineData,omitempty"`
	FunctionCall map[string]interface{} `json:"functionCall,omitempty"`
}

type geminiToolWrap struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

func (c *GeminiConverter) ToCanonical(payload []byte) (*CanonicalRequest, error) {
	var req geminiRequest
	if err := sonic.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("gemini unmarshal: %w", err)
	}
	out := &CanonicalRequest{}
	if req.SystemInstruction != nil {
		for _, p := range req.SystemInstruction.Parts {
			if p.Text != "" {
				if out.System == "" {
					out.System = p.Text
				} else {
					out.System += "\n" + p.Text
				}
			}
		}
	}
	for _, tw := range req.Tools {
		for _, fd := range tw.FunctionDeclarations {
			out.Tools = append(out.Tools, CanonicalTool{
				Name:        fd.Name,
				Description: fd.Description,
				Parameters:  fd.Parameters,
			})
		}
	}
	for _, gc := range req.Contents {
		role := gc.Role
		switch role {
		case "user":
			role = "user"
		case "model":
			role = "assistant"
		default:
			role = "user"
		}
		msg := CanonicalMessage{Role: role}
		for _, p := range gc.Parts {
			if p.Text != "" {
				msg.Content = append(msg.Content, CanonicalPart{Type: "text", Text: p.Text})
			}
			if p.FunctionCall != nil {
				name, _ := p.FunctionCall["name"].(string)
				args, _ := p.FunctionCall["args"].(map[string]interface{})
				msg.Content = append(msg.Content, CanonicalPart{
					Type: "tool_call",
					ToolCall: &CanonicalToolCall{
						Name: name,
						Args: args,
					},
				})
			}
			if p.InlineData != nil {
				msg.Content = append(msg.Content, CanonicalPart{Type: "image"})
			}
		}
		out.Messages = append(out.Messages, msg)
	}
	if req.GenerationConfig != nil {
		if v, ok := req.GenerationConfig["temperature"].(float64); ok {
			out.Temperature = &v
		}
		if v, ok := req.GenerationConfig["topP"].(float64); ok {
			out.TopP = &v
		}
		switch v := req.GenerationConfig["maxOutputTokens"].(type) {
		case float64:
			n := int(v)
			out.MaxTokens = &n
		case int:
			n := v
			out.MaxTokens = &n
		case string:
			if parsed, err := strconv.Atoi(v); err == nil {
				out.MaxTokens = &parsed
			}
		}
	}
	return out, nil
}

func (c *GeminiConverter) FromCanonical(req *CanonicalRequest) ([]byte, error) {
	out := geminiRequest{
		Contents: make([]geminiContent, 0, len(req.Messages)),
	}
	if req.System != "" {
		out.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.System}},
		}
	}
	if len(req.Tools) > 0 {
		wrap := geminiToolWrap{
			FunctionDeclarations: make([]geminiFunctionDeclaration, 0, len(req.Tools)),
		}
		for _, t := range req.Tools {
			wrap.FunctionDeclarations = append(wrap.FunctionDeclarations, geminiFunctionDeclaration{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			})
		}
		out.Tools = append(out.Tools, wrap)
	}
	genCfg := map[string]any{}
	if req.Temperature != nil {
		genCfg["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		genCfg["topP"] = *req.TopP
	}
	if req.MaxTokens != nil {
		genCfg["maxOutputTokens"] = *req.MaxTokens
	}
	if len(genCfg) > 0 {
		out.GenerationConfig = genCfg
	}
	for _, m := range req.Messages {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		gm := geminiContent{Role: role}
		for _, p := range m.Content {
			switch p.Type {
			case "text":
				gm.Parts = append(gm.Parts, geminiPart{Text: p.Text})
			case "tool_call":
				if p.ToolCall == nil {
					continue
				}
				gm.Parts = append(gm.Parts, geminiPart{
					FunctionCall: map[string]interface{}{
						"name": p.ToolCall.Name,
						"args": p.ToolCall.Args,
					},
				})
			}
		}
		out.Contents = append(out.Contents, gm)
	}
	b, err := sonic.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("gemini marshal: %w", err)
	}
	return b, nil
}
