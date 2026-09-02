package controller

import (
	"context"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/samber/lo"
)

// buildTestRequestFromMessages 用捕获的真实消息重建 chat 测活请求。
//
// max_tokens 分支与 buildTestRequest（channel-test.go）保持同步：
// o1 → MaxCompletionTokens 16；model 含 thinking 且非 claude → MaxTokens 50；
// 含 gemini → MaxTokens 3000；否则 MaxTokens = pickTestMaxTokens()。
// 若上游迭代导致两处逻辑背离，以 buildTestRequest 为准，同步更新本函数。
func buildTestRequestFromMessages(testModel string, messages []dto.Message) dto.Request {
	testRequest := &dto.GeneralOpenAIRequest{
		Model:    testModel,
		Stream:   lo.ToPtr(false),
		Messages: messages,
	}

	if dto.IsOpenAIReasoningOModel(testModel) {
		testRequest.MaxCompletionTokens = lo.ToPtr(uint(16))
	} else if strings.Contains(testModel, "thinking") {
		if !strings.Contains(testModel, "claude") {
			testRequest.MaxTokens = lo.ToPtr(uint(50))
		}
	} else if strings.Contains(testModel, "gemini") {
		testRequest.MaxTokens = lo.ToPtr(uint(3000))
	} else {
		testRequest.MaxTokens = lo.ToPtr(pickTestMaxTokens())
	}

	return testRequest
}

// testChannelWithCapturedFallback 真实请求测活统一入口：
// 开关开启且 endpointType == "" 时，优先用捕获消息重建测活（固定非流式）；
// 无捕获、或重建测活失败（localErr/newAPIError 非空）时回退标准构造探测。
func testChannelWithCapturedFallback(
	ctx context.Context,
	channel *model.Channel,
	testUserID int,
	testModel string,
	endpointType string,
	isStream bool,
) testResult {
	if channel != nil && channel.GetCaptureRealRequestTest() && endpointType == "" {
		if messages := GetCapturedTestMessages(channel.Id); len(messages) > 0 {
			result := testChannel(ctx, channel, testUserID, testModel, "", false, messages)
			if result.localErr == nil && result.newAPIError == nil {
				return result
			}
		}
	}
	return testChannel(ctx, channel, testUserID, testModel, endpointType, isStream, nil)
}
