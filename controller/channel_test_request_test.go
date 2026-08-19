package controller

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildTestRequestMessageRandomized 验证 buildTestRequest 对所有含用户消息的
// 测试请求格式都使用随机化的测试问句（而非固定文案），且消息体结构正确。
func TestBuildTestRequestMessageRandomized(t *testing.T) {
	channel := &model.Channel{}

	tests := []struct {
		name         string
		endpointType string
		model        string
	}{
		{name: "openai", endpointType: string(constant.EndpointTypeOpenAI), model: "gpt-4o-mini"},
		{name: "openai-responses", endpointType: string(constant.EndpointTypeOpenAIResponse), model: "gpt-4o"},
		{name: "openai-responses-compact", endpointType: string(constant.EndpointTypeOpenAIResponseCompact), model: "gpt-4o"},
		{name: "claude", endpointType: string(constant.EndpointTypeAnthropic), model: "claude-3-5-sonnet"},
		{name: "gemini", endpointType: string(constant.EndpointTypeGemini), model: "gemini-1.5-pro"},
		{name: "codex-auto", endpointType: "", model: "codex-1"},
		{name: "chat-auto", endpointType: "", model: "gpt-4o"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := buildTestRequest(test.model, test.endpointType, channel, false)

			var message string
			switch typed := req.(type) {
			case *dto.GeneralOpenAIRequest:
				require.Len(t, typed.Messages, 1)
				content, ok := typed.Messages[0].Content.(string)
				require.True(t, ok, "message content should be a string")
				message = content
			case *dto.OpenAIResponsesRequest:
				message = extractResponsesInputMessage(t, typed.Input)
			case *dto.OpenAIResponsesCompactionRequest:
				message = extractResponsesInputMessage(t, typed.Input)
			case *dto.ClaudeRequest:
				require.Len(t, typed.Messages, 1)
				content, ok := typed.Messages[0].Content.(string)
				require.True(t, ok, "claude message content should be a string")
				message = content
			case *dto.GeminiChatRequest:
				require.Len(t, typed.Contents, 1)
				require.Len(t, typed.Contents[0].Parts, 1)
				message = typed.Contents[0].Parts[0].Text
			default:
				t.Fatalf("unexpected request type %T", req)
			}

			assert.Contains(t, testUserMessages, message,
				"test message %q should come from the randomized message pool", message)
		})
	}
}

// TestBuildTestRequestNoFixedProbeText 保护回归：任何测试请求的用户消息都不得
// 再是历史上被上游中转识别为测活的固定文案（"hi"、"彩虹有几种颜色"）。
func TestBuildTestRequestNoFixedProbeText(t *testing.T) {
	for _, probe := range []string{"hi", "彩虹有几种颜色", "ping", "test"} {
		assert.NotContains(t, testUserMessages, probe, "probe text %q must not be in the pool", probe)
	}
}

// TestPickTestUserMessageRandomness 验证随机选择确实会遍历整个消息池，
// 防止池子退化成恒返回同一元素的死代码。
func TestPickTestUserMessageRandomness(t *testing.T) {
	seen := make(map[string]bool, len(testUserMessages))
	for i := 0; i < len(testUserMessages)*50; i++ {
		seen[pickTestUserMessage()] = true
	}
	assert.Len(t, seen, len(testUserMessages), "sampling should eventually cover every pool entry")
}

// TestPickTestMaxTokensWithinRange 验证随机 max_tokens 始终落在配置范围内。
func TestPickTestMaxTokensWithinRange(t *testing.T) {
	for i := 0; i < 100; i++ {
		v := pickTestMaxTokens()
		assert.GreaterOrEqual(t, v, testMaxTokensRange[0])
		assert.LessOrEqual(t, v, testMaxTokensRange[1])
	}
	// 范围端点都应可达，防止范围坍缩
	assert.NotEqual(t, testMaxTokensRange[0], testMaxTokensRange[1], "test max_tokens range should span >1 value")
}

// TestBuildTestRequestMaxTokensRandomized 验证 chat 类测试请求的 max_tokens
// 在范围内随机（不再固定为单一值）。
func TestBuildTestRequestMaxTokensRandomized(t *testing.T) {
	channel := &model.Channel{}
	seen := make(map[uint]bool)
	for i := 0; i < 100; i++ {
		req := buildTestRequest("gpt-4o", string(constant.EndpointTypeOpenAI), channel, false)
		typed, ok := req.(*dto.GeneralOpenAIRequest)
		require.True(t, ok)
		require.NotNil(t, typed.MaxTokens)
		seen[*typed.MaxTokens] = true
		assert.GreaterOrEqual(t, *typed.MaxTokens, testMaxTokensRange[0])
		assert.LessOrEqual(t, *typed.MaxTokens, testMaxTokensRange[1])
	}
	assert.Greater(t, len(seen), 1, "max_tokens should vary across requests")
}

func extractResponsesInputMessage(t *testing.T, input json.RawMessage) string {
	t.Helper()
	var messages []dto.Message
	require.NoError(t, json.Unmarshal(input, &messages))
	require.Len(t, messages, 1)
	content, ok := messages[0].Content.(string)
	require.True(t, ok, "responses input content should be a string")
	return content
}
