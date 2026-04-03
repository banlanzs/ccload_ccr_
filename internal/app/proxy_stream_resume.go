package app

// proxy_stream_resume.go
// 流式响应中断后的无缝续写机制
//
// 问题：上游流不稳定时，流已提交给 CLI（ResponseCommitted=true），
//       无法重发响应头，CLI 会断开。
//
// 解决：
//   1. 流传输时同时缓冲 assistant 已输出的文本（partialText）
//   2. 流中断时，将 partialText 作为 assistant turn 追加到原始 messages
//   3. 用续写请求体向下一个渠道发起请求，直接写入同一个 ResponseWriter
//   4. CLI 感知不到渠道切换，流不中断

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
)

// maxResumeBufferBytes 续写缓冲区上限（防止内存耗尽）
const maxResumeBufferBytes = 512 * 1024 // 512KB

// streamTextCollector 在 SSE 流传输时同时收集 assistant 输出的文本内容
// 支持 OpenAI Chat / Anthropic / Gemini SSE 格式
type streamTextCollector struct {
	buf       bytes.Buffer
	truncated bool

	// 增量 SSE 解析状态
	lineBuf   bytes.Buffer
	eventType string
	dataLines []string
}

// Feed 喂入流数据，提取文本增量
func (c *streamTextCollector) Feed(data []byte) {
	if c.truncated {
		return
	}
	for _, b := range data {
		if b == '\n' {
			line := strings.TrimRight(c.lineBuf.String(), "\r")
			c.lineBuf.Reset()
			c.processLine(line)
		} else {
			c.lineBuf.WriteByte(b)
		}
	}
}

func (c *streamTextCollector) processLine(line string) {
	if after, ok := strings.CutPrefix(line, "event:"); ok {
		c.eventType = strings.TrimSpace(after)
		return
	}
	if after, ok := strings.CutPrefix(line, "data:"); ok {
		dataLine := strings.TrimSpace(after)
		if dataLine == "[DONE]" {
			return
		}
		c.dataLines = append(c.dataLines, dataLine)
		return
	}
	if line == "" && len(c.dataLines) > 0 {
		raw := strings.Join(c.dataLines, "")
		c.dataLines = nil
		c.extractText(c.eventType, raw)
		c.eventType = ""
	}
}

func (c *streamTextCollector) extractText(eventType, data string) {
	// 只处理包含文本增量的事件
	switch eventType {
	case "error", "message_start", "message_stop", "message_delta",
		"content_block_start", "content_block_stop",
		"response.completed", "response.created", "response.in_progress",
		"ping":
		return
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return
	}

	text := ""

	// --- OpenAI Chat format ---
	// {"choices":[{"delta":{"content":"..."}}]}
	if choicesRaw, ok := obj["choices"]; ok {
		var choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(choicesRaw, &choices); err == nil && len(choices) > 0 {
			text = choices[0].Delta.Content
		}
	}

	// --- Anthropic format ---
	// content_block_delta: {"type":"content_block_delta","delta":{"type":"text_delta","text":"..."}}
	if text == "" {
		if deltaRaw, ok := obj["delta"]; ok {
			var delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(deltaRaw, &delta); err == nil && delta.Type == "text_delta" {
				text = delta.Text
			}
		}
	}

	// --- Gemini format ---
	// {"candidates":[{"content":{"parts":[{"text":"..."}]}}]}
	if text == "" {
		if candidatesRaw, ok := obj["candidates"]; ok {
			var candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			}
			if err := json.Unmarshal(candidatesRaw, &candidates); err == nil && len(candidates) > 0 {
				for _, p := range candidates[0].Content.Parts {
					text += p.Text
				}
			}
		}
	}

	if text == "" {
		return
	}

	if c.buf.Len()+len(text) > maxResumeBufferBytes {
		c.truncated = true
		log.Printf("[WARN] [stream-resume] text buffer exceeded %d bytes, resume disabled for this request", maxResumeBufferBytes)
		return
	}
	c.buf.WriteString(text)
}

// Text 返回已收集的 assistant 文本
func (c *streamTextCollector) Text() string {
	return c.buf.String()
}

// Truncated 返回是否因超限而截断
func (c *streamTextCollector) Truncated() bool {
	return c.truncated
}

// ============================================================================
// 续写请求体构建
// ============================================================================

// buildResumeRequestBody 在原始请求体的 messages 末尾追加 partial assistant turn，
// 构造续写请求体。仅支持 OpenAI Chat 格式（messages 数组）。
//
// 返回 nil 表示无法构造续写请求（格式不支持或 partialText 为空）。
func buildResumeRequestBody(originalBody []byte, partialText string) []byte {
	if len(partialText) == 0 {
		return nil
	}

	// 使用 sonic 解析，保留大整数精度
	var reqData map[string]json.RawMessage
	if err := sonic.Unmarshal(originalBody, &reqData); err != nil {
		return nil
	}

	messagesRaw, ok := reqData["messages"]
	if !ok {
		return nil
	}

	var messages []json.RawMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return nil
	}

	// 构造 partial assistant turn
	assistantMsg := map[string]string{
		"role":    "assistant",
		"content": partialText,
	}
	assistantRaw, err := json.Marshal(assistantMsg)
	if err != nil {
		return nil
	}

	messages = append(messages, json.RawMessage(assistantRaw))

	newMessagesRaw, err := json.Marshal(messages)
	if err != nil {
		return nil
	}
	reqData["messages"] = json.RawMessage(newMessagesRaw)

	result, err := sonic.Marshal(reqData)
	if err != nil {
		return nil
	}
	return result
}

// ============================================================================
// 续写用 ResponseWriter 包装器
// ============================================================================

// resumeResponseWriter 包装已提交的 ResponseWriter，
// 忽略 WriteHeader 调用（响应头已发送），直接透传 Write。
// 用于续写场景：渠道切换后继续向同一个连接写入数据。
type resumeResponseWriter struct {
	inner http.ResponseWriter
}

func newResumeResponseWriter(w http.ResponseWriter) *resumeResponseWriter {
	return &resumeResponseWriter{inner: w}
}

func (r *resumeResponseWriter) Header() http.Header {
	return r.inner.Header()
}

// WriteHeader 忽略：响应头已提交，不能重复发送
func (r *resumeResponseWriter) WriteHeader(_ int) {}

func (r *resumeResponseWriter) Write(p []byte) (int, error) {
	return r.inner.Write(p)
}

func (r *resumeResponseWriter) Flush() {
	if f, ok := r.inner.(http.Flusher); ok {
		f.Flush()
	}
}
