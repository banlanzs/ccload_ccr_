package ccr

import (
	"testing"
)

func TestAllConversionPaths(t *testing.T) {
	registry := NewConverterRegistry()

	// 测试所有 6 种转换路径
	paths := []struct {
		name string
		src  ProviderFormat
		dst  ProviderFormat
	}{
		{"OpenAI -> Anthropic", FormatOpenAI, FormatAnthropic},
		{"OpenAI -> Gemini", FormatOpenAI, FormatGemini},
		{"Anthropic -> OpenAI", FormatAnthropic, FormatOpenAI},
		{"Anthropic -> Gemini", FormatAnthropic, FormatGemini},
		{"Gemini -> OpenAI", FormatGemini, FormatOpenAI},
		{"Gemini -> Anthropic", FormatGemini, FormatAnthropic},
	}

	// 使用 OpenAI 格式作为测试载荷（最通用）
	testPayload := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Hello"}
		],
		"temperature": 0.7,
		"max_tokens": 100
	}`)

	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			result, err := registry.Convert(testPayload, p.src, p.dst)
			if err != nil {
				t.Errorf("Conversion failed: %v", err)
				return
			}
			if len(result) == 0 {
				t.Error("Conversion returned empty result")
			}
		})
	}
}

func TestConversionWithTools(t *testing.T) {
	registry := NewConverterRegistry()

	// 测试带 tools 的转换
	openaiPayload := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "What's the weather?"}
		],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "get_weather",
					"description": "Get weather info",
					"parameters": {
						"type": "object",
						"properties": {
							"location": {"type": "string"}
						}
					}
				}
			}
		]
	}`)

	// OpenAI -> Anthropic (tools 转换)
	t.Run("OpenAI -> Anthropic with tools", func(t *testing.T) {
		result, err := registry.Convert(openaiPayload, FormatOpenAI, FormatAnthropic)
		if err != nil {
			t.Errorf("Conversion failed: %v", err)
		}
		if len(result) == 0 {
			t.Error("Conversion returned empty result")
		}
	})

	// OpenAI -> Gemini (tools 转换)
	t.Run("OpenAI -> Gemini with tools", func(t *testing.T) {
		result, err := registry.Convert(openaiPayload, FormatOpenAI, FormatGemini)
		if err != nil {
			t.Errorf("Conversion failed: %v", err)
		}
		if len(result) == 0 {
			t.Error("Conversion returned empty result")
		}
	})
}
