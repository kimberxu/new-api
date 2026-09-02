package controller

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractTestMessages(t *testing.T) {
	t.Run("nil request", func(t *testing.T) {
		require.Nil(t, extractTestMessages(nil))
	})

	t.Run("empty messages", func(t *testing.T) {
		require.Nil(t, extractTestMessages(&dto.GeneralOpenAIRequest{}))
	})

	t.Run("skip tool role and tool_calls assistant", func(t *testing.T) {
		req := &dto.GeneralOpenAIRequest{
			Messages: []dto.Message{
				{Role: "system", Content: "sys"},
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: "calling tool", ToolCalls: json.RawMessage(`[{"id":"1"}]`)},
				{Role: "tool", Content: "tool result"},
				{Role: "assistant", Content: "final answer"},
				{Role: "user", Content: "second question"},
			},
		}
		result := extractTestMessages(req)
		// valid: sys, hello, final answer, second question (4条) → 终点 second question → 全取
		require.Len(t, result, 4)
		assert.Equal(t, "system", result[0].Role)
		assert.Equal(t, "sys", result[0].Content)
		assert.Equal(t, "user", result[1].Role)
		assert.Equal(t, "hello", result[1].Content)
		assert.Equal(t, "assistant", result[2].Role)
		assert.Equal(t, "final answer", result[2].Content)
		assert.Equal(t, "user", result[3].Role)
		assert.Equal(t, "second question", result[3].Content)
	})

	t.Run("multimodal content extracts only text parts", func(t *testing.T) {
		req := &dto.GeneralOpenAIRequest{
			Messages: []dto.Message{
				{
					Role: "user",
					Content: []any{
						dto.MediaContent{Type: dto.ContentTypeImageURL, ImageUrl: map[string]any{"url": "https://example.com/a.png"}},
						dto.MediaContent{Type: dto.ContentTypeText, Text: "describe this image"},
					},
				},
			},
		}
		result := extractTestMessages(req)
		require.Len(t, result, 1)
		assert.Equal(t, "describe this image", result[0].Content)
	})

	t.Run("truncate long text", func(t *testing.T) {
		longText := strings.Repeat("测", capturedMessageMaxLen+100)
		req := &dto.GeneralOpenAIRequest{
			Messages: []dto.Message{
				{Role: "user", Content: longText},
			},
		}
		result := extractTestMessages(req)
		require.Len(t, result, 1)
		runes := []rune(result[0].Content.(string))
		assert.Len(t, runes, capturedMessageMaxLen)
	})

	t.Run("no user message returns nil", func(t *testing.T) {
		req := &dto.GeneralOpenAIRequest{
			Messages: []dto.Message{
				{Role: "system", Content: "you are helpful"},
				{Role: "assistant", Content: "ok"},
			},
		}
		require.Nil(t, extractTestMessages(req))
	})

	t.Run("no text content returns nil", func(t *testing.T) {
		req := &dto.GeneralOpenAIRequest{
			Messages: []dto.Message{
				{
					Role: "user",
					Content: []any{
						dto.MediaContent{Type: dto.ContentTypeImageURL, ImageUrl: map[string]any{"url": "https://example.com/a.png"}},
					},
				},
			},
		}
		require.Nil(t, extractTestMessages(req))
	})

	t.Run("takes at most 4 messages ending at last user message", func(t *testing.T) {
		req := &dto.GeneralOpenAIRequest{
			Messages: []dto.Message{
				{Role: "system", Content: "sys prompt"},
				{Role: "user", Content: "q1"},
				{Role: "assistant", Content: "a1"},
				{Role: "user", Content: "q2"},
				{Role: "assistant", Content: "a2"},
				{Role: "user", Content: "q3"},
				{Role: "assistant", Content: "after last user"},
			},
		}
		result := extractTestMessages(req)
		// valid(排除 tool_calls/tool): sys(0),q1(1),a1(2),q2(3),a2(4),q3(5),after(6)
		// lastUserIdx = 5 (q3), start = 5-3 = 2 → [a1, q2, a2, q3]
		require.Len(t, result, 4)
		assert.Equal(t, "assistant", result[0].Role)
		assert.Equal(t, "a1", result[0].Content)
		assert.Equal(t, "user", result[1].Role)
		assert.Equal(t, "q2", result[1].Content)
		assert.Equal(t, "assistant", result[2].Role)
		assert.Equal(t, "a2", result[2].Content)
		assert.Equal(t, "user", result[3].Role)
		assert.Equal(t, "q3", result[3].Content)
	})
}

func TestCapturedPoolRingBuffer(t *testing.T) {
	channelID := 9001
	ClearCapturedTestMessages(channelID)

	for i := 0; i < capturedMessageMaxCount+1; i++ {
		req := &dto.GeneralOpenAIRequest{
			Messages: []dto.Message{
				{Role: "user", Content: string(rune('A' + i))},
			},
		}
		CaptureTestMessages(channelID, req)
	}

	globalCapturedPool.mu.RLock()
	pool := globalCapturedPool.pool[channelID]
	globalCapturedPool.mu.RUnlock()

	require.Len(t, pool, capturedMessageMaxCount)
	// 最旧的 'A' 已被移除，最早保留 'B'
	assert.Equal(t, "B", pool[0][0].Content)
	assert.Equal(t, string(rune('A'+capturedMessageMaxCount)), pool[capturedMessageMaxCount-1][0].Content)

	ClearCapturedTestMessages(channelID)
}

func TestCapturedPoolRandomPick(t *testing.T) {
	channelID := 9002
	ClearCapturedTestMessages(channelID)

	texts := []string{"alpha", "beta", "gamma", "delta"}
	for _, txt := range texts {
		req := &dto.GeneralOpenAIRequest{
			Messages: []dto.Message{{Role: "user", Content: txt}},
		}
		CaptureTestMessages(channelID, req)
	}

	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		msgs := GetCapturedTestMessages(channelID)
		require.NotNil(t, msgs)
		seen[msgs[0].Content.(string)] = true
		if len(seen) == len(texts) {
			break
		}
	}
	assert.Len(t, seen, len(texts), "random pick should cover all entries over 200 draws")

	// 无捕获时返回 nil
	require.Nil(t, GetCapturedTestMessages(999999))

	ClearCapturedTestMessages(channelID)
}

func TestCaptureTestMessagesIgnoresNonChatRequest(t *testing.T) {
	channelID := 9003
	ClearCapturedTestMessages(channelID)

	// embedding request implements dto.Request but is not *dto.GeneralOpenAIRequest
	CaptureTestMessages(channelID, &dto.EmbeddingRequest{Model: "text-embedding-3-small", Input: []any{"hello"}})
	CaptureTestMessages(channelID, nil)
	require.Nil(t, GetCapturedTestMessages(channelID))

	ClearCapturedTestMessages(channelID)
}

func TestBuildTestRequestFromMessages(t *testing.T) {
	msgs := []dto.Message{{Role: "user", Content: "你好"}}

	t.Run("o1 model uses MaxCompletionTokens and non-stream", func(t *testing.T) {
		req := buildTestRequestFromMessages("o1-mini", msgs)
		openaiReq, ok := req.(*dto.GeneralOpenAIRequest)
		require.True(t, ok)
		assert.Equal(t, "o1-mini", openaiReq.Model)
		require.NotNil(t, openaiReq.Stream)
		assert.False(t, *openaiReq.Stream)
		require.NotNil(t, openaiReq.MaxCompletionTokens)
		assert.Equal(t, uint(16), *openaiReq.MaxCompletionTokens)
		assert.Nil(t, openaiReq.MaxTokens)
	})

	t.Run("plain model MaxTokens within range", func(t *testing.T) {
		req := buildTestRequestFromMessages("gpt-4o-mini", msgs)
		openaiReq, ok := req.(*dto.GeneralOpenAIRequest)
		require.True(t, ok)
		require.NotNil(t, openaiReq.MaxTokens)
		assert.GreaterOrEqual(t, *openaiReq.MaxTokens, testMaxTokensRange[0])
		assert.LessOrEqual(t, *openaiReq.MaxTokens, testMaxTokensRange[1])
		assert.Nil(t, openaiReq.MaxCompletionTokens)
	})

	t.Run("thinking non-claude model gets MaxTokens 50", func(t *testing.T) {
		req := buildTestRequestFromMessages("deepseek-thinking", msgs)
		openaiReq, ok := req.(*dto.GeneralOpenAIRequest)
		require.True(t, ok)
		require.NotNil(t, openaiReq.MaxTokens)
		assert.Equal(t, uint(50), *openaiReq.MaxTokens)
	})

	t.Run("gemini model gets MaxTokens 3000", func(t *testing.T) {
		req := buildTestRequestFromMessages("gemini-2.0-flash", msgs)
		openaiReq, ok := req.(*dto.GeneralOpenAIRequest)
		require.True(t, ok)
		require.NotNil(t, openaiReq.MaxTokens)
		assert.Equal(t, uint(3000), *openaiReq.MaxTokens)
	})

	t.Run("rebuilt request carries no reasoning or tool fields", func(t *testing.T) {
		req := buildTestRequestFromMessages("gpt-4o", msgs)
		openaiReq, ok := req.(*dto.GeneralOpenAIRequest)
		require.True(t, ok)
		assert.Empty(t, openaiReq.ReasoningEffort)
		for _, m := range openaiReq.Messages {
			assert.Empty(t, m.ToolCalls)
			assert.Nil(t, m.ReasoningContent)
			assert.Nil(t, m.Reasoning)
		}
	})
}
