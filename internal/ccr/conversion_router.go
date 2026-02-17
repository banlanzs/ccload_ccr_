package ccr

import (
	"errors"
	"fmt"
	"strings"
)

// 转换错误类型
var (
	// ErrNotApplicable 表示转换器不适用于此转换路径（非错误，可降级）
	ErrNotApplicable = errors.New("conversion not applicable for this route")
	// ErrTransformFailed 表示转换执行失败（真实错误）
	ErrTransformFailed = errors.New("conversion transform failed")
)

// ConversionRouter routes conversion between:
// 1) legacy transformer.go path (kept untouched), and
// 2) new canonical converter system.
type ConversionRouter struct {
	registry *ConverterRegistry
}

func NewConversionRouter(registry *ConverterRegistry) *ConversionRouter {
	if registry == nil {
		registry = NewConverterRegistry()
	}
	return &ConversionRouter{registry: registry}
}

// Route converts payload between provider formats.
// preferLegacy=true will route OpenAI<->Anthropic via legacy transformer semantics.
// Any path involving Gemini always uses canonical converters.
func (r *ConversionRouter) Route(payload []byte, source, target string, preferLegacy bool) ([]byte, error) {
	src := ParseProviderFormat(strings.ToLower(strings.TrimSpace(source)))
	dst := ParseProviderFormat(strings.ToLower(strings.TrimSpace(target)))
	if src == "" || dst == "" {
		return nil, fmt.Errorf("invalid conversion route: source=%q target=%q", source, target)
	}
	if src == dst {
		return payload, nil // 无需转换
	}

	// 决策树：
	// 1. 如果涉及 Gemini，强制使用新系统（Legacy 不支持 Gemini）
	// 2. 如果 preferLegacy=true 且支持 Legacy，优先 Legacy → 失败时回落新系统
	// 3. 否则尝试新系统 → 失败时回落 Legacy

	if src == FormatGemini || dst == FormatGemini {
		// Gemini 必须用新系统
		return r.registry.Convert(payload, src, dst)
	}

	if preferLegacy && isLegacyConvertible(src, dst) {
		// 优先 Legacy
		result, err := r.routeLegacy(payload, src, dst)
		if err == nil {
			return result, nil
		}
		// Legacy 失败，检查是否可回落
		if !errors.Is(err, ErrNotApplicable) {
			// 真实错误（TransformFailed），尝试回落新系统
			if newResult, newErr := r.registry.Convert(payload, src, dst); newErr == nil {
				return newResult, nil
			}
		}
		// 无法回落，返回简化错误（避免泄露内部细节）
		return nil, fmt.Errorf("%w: conversion failed for %s->%s", ErrTransformFailed, src, dst)
	}

	// 尝试新系统
	result, err := r.registry.Convert(payload, src, dst)
	if err == nil {
		return result, nil
	}

	// 新系统失败，尝试回落 Legacy
	if isLegacyConvertible(src, dst) {
		legacyResult, legacyErr := r.routeLegacy(payload, src, dst)
		if legacyErr == nil {
			return legacyResult, nil
		}
		// Legacy 也失败，返回简化错误
		return nil, fmt.Errorf("%w: conversion failed for %s->%s", ErrTransformFailed, src, dst)
	}

	return nil, err
}

func isLegacyConvertible(src, dst ProviderFormat) bool {
	return (src == FormatOpenAI && dst == FormatAnthropic) || (src == FormatAnthropic && dst == FormatOpenAI)
}

func (r *ConversionRouter) routeLegacy(payload []byte, src, dst ProviderFormat) ([]byte, error) {
	// Legacy 系统仅支持 OpenAI → Anthropic 单向转换
	if src != FormatOpenAI || dst != FormatAnthropic {
		// 不支持的转换方向，返回 NotApplicable（非错误，可降级）
		return nil, fmt.Errorf("%w: %s->%s", ErrNotApplicable, src, dst)
	}

	// 获取 transformer 实例
	transformer, err := GetTransformer("openai_to_claude")
	if err != nil {
		return nil, fmt.Errorf("get legacy transformer: %w", err)
	}

	// 执行转换
	result, err := transformer.TransformRequest(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: legacy transformer failed: %v", ErrTransformFailed, err)
	}

	return result, nil
}
