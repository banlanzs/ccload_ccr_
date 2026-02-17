package ccr

import (
	"errors"
	"testing"
)

func TestRoute_InvalidRoute(t *testing.T) {
	router := NewConversionRouter(nil)
	_, err := router.Route([]byte(`{}`), "bad", "anthropic", false)
	if err == nil {
		t.Fatalf("expected invalid route error")
	}
}

func TestRoute_SameSourceTarget_ReturnsOriginal(t *testing.T) {
	router := NewConversionRouter(nil)
	in := []byte(`{"x":1}`)
	out, err := router.Route(in, "openai", "openai", false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(out) != string(in) {
		t.Fatalf("expected original payload")
	}
}

func TestRouteLegacy_UnsupportedRoute(t *testing.T) {
	router := NewConversionRouter(nil)
	_, err := router.routeLegacy([]byte(`{}`), FormatGemini, FormatOpenAI)
	if err == nil {
		t.Fatalf("expected not applicable error")
	}
	if !errors.Is(err, ErrNotApplicable) {
		t.Fatalf("expected ErrNotApplicable, got: %v", err)
	}
}

func TestRouteLegacy_OpenAIToAnthropic(t *testing.T) {
	router := NewConversionRouter(nil)
	// Valid OpenAI format payload
	payload := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`)
	out, err := router.routeLegacy(payload, FormatOpenAI, FormatAnthropic)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected transformed payload")
	}
}

func TestRouteLegacy_AnthropicToOpenAI_NotSupported(t *testing.T) {
	router := NewConversionRouter(nil)
	// Legacy 系统不支持反向转换
	payload := []byte(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hello"}]}`)
	_, err := router.routeLegacy(payload, FormatAnthropic, FormatOpenAI)
	if err == nil {
		t.Fatalf("expected not applicable error")
	}
	if !errors.Is(err, ErrNotApplicable) {
		t.Fatalf("expected ErrNotApplicable, got: %v", err)
	}
}

func TestErrorWrappers_IsSemantics(t *testing.T) {
	e1 := &wrapError{msg: "test", inner: errors.New("inner"), errType: ErrNotApplicable}
	if !errors.Is(e1, ErrNotApplicable) {
		t.Fatalf("expected ErrNotApplicable")
	}

	e2 := &wrapError{msg: "test", inner: errors.New("inner"), errType: ErrTransformFailed}
	if !errors.Is(e2, ErrTransformFailed) {
		t.Fatalf("expected ErrTransformFailed")
	}
}

// wrapError is a helper for testing error wrapping
type wrapError struct {
	msg     string
	inner   error
	errType error
}

func (e *wrapError) Error() string {
	return e.msg
}

func (e *wrapError) Unwrap() error {
	return e.inner
}

func (e *wrapError) Is(target error) bool {
	if target == ErrNotApplicable || target == ErrTransformFailed {
		return e.errType == target
	}
	return false
}
