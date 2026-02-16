package ccr

import (
	"fmt"
	"strings"
)

// Transformer 定义协议转换器接口
type Transformer interface {
	Name() string
	TransformRequest(body []byte) ([]byte, error)
}

// ResponseTransformer 定义响应转换器接口
type ResponseTransformer interface {
	Name() string
	TransformResponse(body []byte) ([]byte, error)
	TransformStreamEvent(event []byte) ([]byte, error)
}

// GetTransformer 根据名称获取转换器实例
func GetTransformer(name string) (Transformer, error) {
	key := strings.TrimSpace(strings.ToLower(name))
	if key == "" {
		return nil, fmt.Errorf("transformer name is empty")
	}

	switch key {
	case "openai_to_claude":
		return &OpenAIToClaude{}, nil
	default:
		return nil, fmt.Errorf("unsupported transformer: %s", name)
	}
}

// GetResponseTransformer 根据名称获取响应转换器实例
func GetResponseTransformer(name string) (ResponseTransformer, error) {
	key := strings.TrimSpace(strings.ToLower(name))
	if key == "" {
		return nil, fmt.Errorf("transformer name is empty")
	}

	switch key {
	case "openai_to_claude":
		// 请求转换是 OpenAI → Claude，响应转换是 Claude → OpenAI
		return &ClaudeToOpenAI{}, nil
	default:
		return nil, fmt.Errorf("unsupported response transformer: %s", name)
	}
}
