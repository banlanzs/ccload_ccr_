package ccr

import (
	"fmt"
)

type Converter interface {
	ToCanonical(payload []byte) (*CanonicalRequest, error)
	FromCanonical(req *CanonicalRequest) ([]byte, error)
}

type ConverterRegistry struct {
	converters map[ProviderFormat]Converter
}

func NewConverterRegistry() *ConverterRegistry {
	r := &ConverterRegistry{
		converters: make(map[ProviderFormat]Converter, 3),
	}
	r.Register(FormatOpenAI, NewOpenAIConverter())
	r.Register(FormatAnthropic, NewAnthropicConverter())
	r.Register(FormatGemini, NewGeminiConverter())
	return r
}

func (r *ConverterRegistry) Register(format ProviderFormat, converter Converter) {
	if r == nil || format == "" || converter == nil {
		return
	}
	r.converters[format] = converter
}

func (r *ConverterRegistry) Get(format ProviderFormat) (Converter, error) {
	if r == nil {
		return nil, fmt.Errorf("converter registry is nil")
	}
	c, ok := r.converters[format]
	if !ok {
		return nil, fmt.Errorf("unsupported provider format: %s", format)
	}
	return c, nil
}

func (r *ConverterRegistry) Convert(payload []byte, source, target ProviderFormat) ([]byte, error) {
	src, err := r.Get(source)
	if err != nil {
		return nil, fmt.Errorf("get source converter %q: %w", source, err)
	}
	dst, err := r.Get(target)
	if err != nil {
		return nil, fmt.Errorf("get target converter %q: %w", target, err)
	}
	canonical, err := src.ToCanonical(payload)
	if err != nil {
		return nil, fmt.Errorf("to canonical (%s): %w", source, err)
	}
	out, err := dst.FromCanonical(canonical)
	if err != nil {
		return nil, fmt.Errorf("from canonical (%s): %w", target, err)
	}
	return out, nil
}
