package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withChatResponsesPolicy(t *testing.T, policy model_setting.ChatCompletionsToResponsesPolicy, fn func()) {
	t.Helper()
	original := model_setting.GetGlobalSettings().ChatCompletionsToResponsesPolicy
	model_setting.GetGlobalSettings().ChatCompletionsToResponsesPolicy = policy
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().ChatCompletionsToResponsesPolicy = original
	})
	fn()
}

// The failure behind this test: downstream asked for a routable group name
// (all-text-only) while the policy only matched the real upstream model
// (muse-spark-...). Judging the policy by OriginModelName skips the
// chat→responses conversion and the responses-only upstream 500s.
func TestChatResponsesPolicyJudgesOutboundModel(t *testing.T) {
	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   false,
		ChannelIDs:    []int{53},
		ModelPatterns: []string{"muse-spark"},
	}
	withChatResponsesPolicy(t, policy, func() {
		require.True(t, ShouldChatCompletionsUseResponsesGlobal(53, 1, "muse-spark-1.3-contributor-free"))
		assert.False(t, ShouldChatCompletionsUseResponsesGlobal(53, 1, "all-text-only"))
	})
}

func TestChatResponsesPolicyRespectsChannelGate(t *testing.T) {
	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   false,
		ChannelIDs:    []int{53},
		ModelPatterns: []string{"muse-spark"},
	}
	withChatResponsesPolicy(t, policy, func() {
		assert.False(t, ShouldChatCompletionsUseResponsesGlobal(49, 1, "muse-spark-1.3-contributor-free"))
	})
}
