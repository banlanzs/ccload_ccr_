package ccr

import (
	"testing"
)

func TestParseProviderFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected ProviderFormat
	}{
		{"openai", FormatOpenAI},
		{"OpenAI", FormatOpenAI},
		{"OPENAI", FormatOpenAI},
		{"anthropic", FormatAnthropic},
		{"Anthropic", FormatAnthropic},
		{"claude", FormatAnthropic},
		{"gemini", FormatGemini},
		{"Gemini", FormatGemini},
		{"", ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseProviderFormat(tt.input)
			if result != tt.expected {
				t.Errorf("ParseProviderFormat(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDetectFormatFromPayload(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected ProviderFormat
	}{
		{
			name:     "Anthropic format",
			payload:  `{"anthropic_version":"2023-06-01","messages":[{"role":"user","content":"Hello"}]}`,
			expected: FormatAnthropic,
		},
		{
			name:     "Gemini format",
			payload:  `{"contents":[{"parts":[{"text":"Hello"}]}]}`,
			expected: FormatGemini,
		},
		{
			name:     "OpenAI format",
			payload:  `{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`,
			expected: FormatOpenAI,
		},
		{
			name:     "Empty payload",
			payload:  "",
			expected: "",
		},
		{
			name:     "Unknown format",
			payload:  `{"data":"value"}`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFormatFromPayload([]byte(tt.payload))
			if result != tt.expected {
				t.Errorf("DetectFormatFromPayload() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestInferFormatFromChannelType(t *testing.T) {
	tests := []struct {
		input    string
		expected ProviderFormat
	}{
		{"anthropic", FormatAnthropic},
		{"Anthropic", FormatAnthropic},
		{"codex", FormatAnthropic},
		{"openai", FormatOpenAI},
		{"OpenAI", FormatOpenAI},
		{"gemini", FormatGemini},
		{"Gemini", FormatGemini},
		{"", ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := InferFormatFromChannelType(tt.input)
			if result != tt.expected {
				t.Errorf("InferFormatFromChannelType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNeedsConversion(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		channelType string
		expected    bool
	}{
		{
			name:        "OpenAI to Anthropic needs conversion",
			payload:     `{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`,
			channelType: "anthropic",
			expected:    true,
		},
		{
			name:        "Same format no conversion",
			payload:     `{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`,
			channelType: "openai",
			expected:    false,
		},
		{
			name:        "Unable to detect source format",
			payload:     `{"data":"value"}`,
			channelType: "anthropic",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsConversion([]byte(tt.payload), tt.channelType)
			if result != tt.expected {
				t.Errorf("NeedsConversion() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConverterRegistry(t *testing.T) {
	registry := NewConverterRegistry()

	// Test that all expected converters are registered
	formats := []ProviderFormat{FormatOpenAI, FormatAnthropic, FormatGemini}
	for _, format := range formats {
		t.Run(string(format), func(t *testing.T) {
			converter, err := registry.Get(format)
			if err != nil {
				t.Errorf("Get(%q) error = %v", format, err)
			}
			if converter == nil {
				t.Errorf("Get(%q) returned nil converter", format)
			}
		})
	}

	// Test unsupported format
	t.Run("unsupported format", func(t *testing.T) {
		_, err := registry.Get("unsupported")
		if err == nil {
			t.Error("Get(unsupported) expected error, got nil")
		}
	})
}

func TestConversionRouter(t *testing.T) {
	router := NewConversionRouter(nil)

	// Test no-op when source == target
	payload := []byte(`{"test":"data"}`)
	result, err := router.Route(payload, "openai", "openai", false)
	if err != nil {
		t.Errorf("Route(same format) error = %v", err)
	}
	if string(result) != string(payload) {
		t.Error("Route(same format) modified payload")
	}

	// Test invalid format
	t.Run("invalid format", func(t *testing.T) {
		_, err := router.Route(payload, "", "openai", false)
		if err == nil {
			t.Error("Route(invalid source) expected error, got nil")
		}
	})
}
