package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withTestPolicy(t *testing.T, policy model_setting.ChatCompletionsToResponsesPolicy, fn func()) {
	t.Helper()
	orig := model_setting.GetGlobalSettings().ChatCompletionsToResponsesPolicy
	model_setting.GetGlobalSettings().ChatCompletionsToResponsesPolicy = policy
	prevCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().ChatCompletionsToResponsesPolicy = orig
		common.MemoryCacheEnabled = prevCache
	})
	fn()
}

func channelWithMappingJSON(t *testing.T, channelID int, mappingJSON string) *model.Channel {
	t.Helper()
	mapping := mappingJSON
	return &model.Channel{
		Id:           channelID,
		Type:         constant.ChannelTypeOpenAI,
		ModelMapping: &mapping,
	}
}

// Downstream asked for all-text-only but the real upstream needs /v1/responses.
// Before the fix isInbound check: shouldForceResponsesForTest(channel, "all-text-only")
// returned false; the health probe sent /v1/chat/completions and got the same 500
// real traffic did.
func TestShouldForceResponsesForTest_UnifiesToOutbound_GroupNameMapping(t *testing.T) {
	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   false,
		ChannelIDs:    []int{53},
		ModelPatterns: []string{"muse-spark"},
	}
	withTestPolicy(t, policy, func() {
		ch := channelWithMappingJSON(t, 53, `{"all-text-only":"muse-spark-1.3-contributor-free"}`)
		require.True(t, shouldForceResponsesForTest(ch, "all-text-only"))
	})
}

// 1:N weighted map: one branch needs responses, so the probe must use responses.
func TestShouldForceResponsesForTest_WeightedCandidateAnyMatch(t *testing.T) {
	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   false,
		ChannelIDs:    []int{53},
		ModelPatterns: []string{"muse-spark"},
	}
	withTestPolicy(t, policy, func() {
		ch := channelWithMappingJSON(t, 53, `{"all-text-only":[
			{"model":"gpt-4o-mini","weight":1},
			{"model":"muse-spark-1.3-contributor-free","weight":1}
		]}`)
		require.True(t, shouldForceResponsesForTest(ch, "all-text-only"))
	})
}

// Reasoning suffix is stripped the same way real handlers do: gpt-5-high maps to gpt-5,
// so a pattern on the base must still match.
func TestShouldForceResponsesForTest_StripsReasoningSuffix(t *testing.T) {
	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   true,
		ModelPatterns: []string{"^gpt-5$"},
	}
	withTestPolicy(t, policy, func() {
		ch := channelWithMappingJSON(t, 53, `{"gpt-5-high":"gpt-5"}`)
		require.True(t, shouldForceResponsesForTest(ch, "gpt-5-high"))
	})
}

func TestShouldForceResponsesForTest_PassThroughNeverForces(t *testing.T) {
	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   true,
		ModelPatterns: []string{"muse-spark"},
	}
	withTestPolicy(t, policy, func() {
		passThrough := `{"pass_through_body_enabled":true}`
		ch := &model.Channel{
			Id:           53,
			Type:         constant.ChannelTypeOpenAI,
			Setting:      &passThrough,
			ModelMapping: lo.ToPtr(`{"all-text-only":"muse-spark-1.3-contributor-free"}`),
		}
		assert.False(t, shouldForceResponsesForTest(ch, "all-text-only"))
	})
}

func TestShouldForceResponsesForTest_OpenRouterThinkingStripped(t *testing.T) {
	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   true,
		ModelPatterns: []string{`^qwen2\.5-7b-instruct$`, `^claude-3-5-sonnet$`},
	}
	withTestPolicy(t, policy, func() {
		// 7c044d7c5 BREAKING: OpenRouter generic "-thinking" alias removed.
		// Only whitelisted families (gpt-*/o-series, claude-*, gemini-*) strip
		// -thinking; qwen-max and similar stay opaque. So qwen2.5 on
		// OpenRouter no longer strips and neither path should force responses.
		ch := &model.Channel{
			Id:           53,
			Type:         constant.ChannelTypeOpenRouter,
			ModelMapping: lo.ToPtr(`{"all-text-only":"qwen2.5-7b-instruct-thinking"}`),
		}
		assert.False(t, shouldForceResponsesForTest(ch, "all-text-only"))
		chNonOR := &model.Channel{
			Id:           53,
			Type:         constant.ChannelTypeOpenAI,
			ModelMapping: lo.ToPtr(`{"all-text-only":"qwen2.5-7b-instruct-thinking"}`),
		}
		assert.False(t, shouldForceResponsesForTest(chNonOR, "all-text-only"))
		// Whitelisted family still strips: claude-3-5-sonnet-thinking -> claude-3-5-sonnet on OpenRouter.
		chWhitelisted := &model.Channel{
			Id:           53,
			Type:         constant.ChannelTypeOpenRouter,
			ModelMapping: lo.ToPtr(`{"all-text-only":"claude-3-5-sonnet-thinking"}`),
		}
		assert.True(t, shouldForceResponsesForTest(chWhitelisted, "all-text-only"))
	})
}
func TestCandidateOutboundModelsExpandsMappingChain(t *testing.T) {
	prevCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = prevCache })
	ch := channelWithMappingJSON(t, 53, `{"a":"b","b":"c"}`)
	got := candidateOutboundModelsForTest(ch, "a")
	assert.Equal(t, []string{"c"}, got)
}
