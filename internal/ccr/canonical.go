package ccr

import "strings"

// CanonicalRequest is an internal neutral schema used by the new
// converter pipeline. It coexists with legacy transformer.go flow.
type CanonicalRequest struct {
	Model       string                 `json:"model,omitempty"`
	System      string                 `json:"system,omitempty"`
	Messages    []CanonicalMessage     `json:"messages,omitempty"`
	Tools       []CanonicalTool        `json:"tools,omitempty"`
	ToolChoice  any                    `json:"tool_choice,omitempty"`
	Temperature *float64               `json:"temperature,omitempty"`
	TopP        *float64               `json:"top_p,omitempty"`
	MaxTokens   *int                   `json:"max_tokens,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type CanonicalMessage struct {
	Role    string          `json:"role"`
	Content []CanonicalPart `json:"content,omitempty"`
	Name    string          `json:"name,omitempty"`
}

type CanonicalPart struct {
	Type     string                 `json:"type"`
	Text     string                 `json:"text,omitempty"`
	Data     string                 `json:"data,omitempty"`
	MimeType string                 `json:"mime_type,omitempty"`
	Name     string                 `json:"name,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
	ToolCall *CanonicalToolCall     `json:"tool_call,omitempty"`
}

type CanonicalTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type CanonicalToolCall struct {
	ID   string                 `json:"id,omitempty"`
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args,omitempty"`
}

type ProviderFormat string

const (
	FormatOpenAI    ProviderFormat = "openai"
	FormatAnthropic ProviderFormat = "anthropic"
	FormatGemini    ProviderFormat = "gemini"
)

func ParseProviderFormat(v string) ProviderFormat {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case string(FormatOpenAI):
		return FormatOpenAI
	case string(FormatAnthropic), "claude":
		return FormatAnthropic
	case string(FormatGemini):
		return FormatGemini
	default:
		return ""
	}
}
