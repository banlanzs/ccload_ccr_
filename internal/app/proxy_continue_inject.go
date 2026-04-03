package app

// proxy_continue_inject.go
// 当上游模型因 context window 限制提前结束输出（finish_reason=length / stop_reason=max_tokens）时，
// 向 CLI 注入一个额外的 SSE chunk，内容为 "\n\ncontinue"，
// 让 CLI（如 claude code）自动触发继续工作，无需用户手动干预。

import (
	"fmt"
	"net/http"
	"time"
)

// continueText 注入的提示文本
const continueText = "\n\ncontinue"

// injectContinueChunk 向已提交的 SSE 流写入一个 continue 提示 chunk。
// 根据渠道类型生成对应格式的 SSE 事件。
func injectContinueChunk(w http.ResponseWriter, channelType string) {
	var chunk string
	ts := time.Now().UnixMilli()

	switch channelType {
	case "anthropic":
		// Anthropic content_block_delta 格式
		chunk = fmt.Sprintf(
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n",
			continueText,
		)
	default:
		// OpenAI Chat Completions delta 格式（openai / codex / gemini 转换后均适用）
		chunk = fmt.Sprintf(
			"data: {\"id\":\"chatcmpl-continue-%d\",\"object\":\"chat.completion.chunk\",\"created\":%d,\"model\":\"continue\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%q},\"finish_reason\":null}]}\n\n",
			ts, ts/1000, continueText,
		)
	}

	_, _ = fmt.Fprint(w, chunk)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
