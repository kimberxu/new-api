package service

import (
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetDisableWindowConfig restores the default sliding window parameters so
// tests are deterministic regardless of execution order.
func resetDisableWindowConfig() {
	common.ConfiguredDisableWindowSeconds = 600
	common.ConfiguredDisableThreshold = 2
	common.UnconfiguredDisableWindowSeconds = 300
	common.UnconfiguredDisableThreshold = 3
}

// resetDisableWindowLimiter replaces the singleton InMemoryRateLimiter so each
// test starts with a clean store.
func resetDisableWindowLimiter() {
	channelDisableWindowMemoryLimiterOnce = sync.Once{}
	channelDisableWindowMemoryLimiter = nil
	_ = getChannelDisableWindowMemoryLimiter()
}

// recordDisable wraps CheckAndRecordDisable for boolean assertions; the
// trigger detail is covered separately in TestShouldDisableChannelWithDecision.
func recordDisable(channelID int, statusCode int, isConfiguredError bool) bool {
	triggered, _ := CheckAndRecordDisable(channelID, statusCode, isConfiguredError)
	return triggered
}

func TestCheckAndRecordDisable_ConfiguredThreshold(t *testing.T) {
	resetDisableWindowConfig()
	common.RedisEnabled = false
	resetDisableWindowLimiter()

	// Default: configured threshold = 2, window = 600s.
	// 1st error: should NOT trigger.
	assert.False(t, recordDisable(1, 500, true), "first configured error should not trigger disable")
	// 2nd error: should trigger.
	assert.True(t, recordDisable(1, 500, true), "second configured error should trigger disable")
}

func TestCheckAndRecordDisable_UnconfiguredThreshold(t *testing.T) {
	resetDisableWindowConfig()
	common.RedisEnabled = false
	resetDisableWindowLimiter()

	// Default: unconfigured threshold = 3, window = 300s.
	assert.False(t, recordDisable(1, 503, false), "1st unconfigured error should not trigger")
	assert.False(t, recordDisable(1, 503, false), "2nd unconfigured error should not trigger")
	assert.True(t, recordDisable(1, 503, false), "3rd unconfigured error should trigger")
}

func TestCheckAndRecordDisable_DifferentStatusCodesIndependent(t *testing.T) {
	resetDisableWindowConfig()
	common.RedisEnabled = false
	resetDisableWindowLimiter()

	// Channel 1, status 500 — 1 configured error.
	assert.False(t, recordDisable(1, 500, true))
	// Channel 1, status 502 — different key, should start fresh.
	assert.False(t, recordDisable(1, 502, true))
	// Channel 1, status 500 — 2nd error, should trigger.
	assert.True(t, recordDisable(1, 500, true))
}

func TestCheckAndRecordDisable_DifferentChannelsIndependent(t *testing.T) {
	resetDisableWindowConfig()
	common.RedisEnabled = false
	resetDisableWindowLimiter()

	// Channel 1, status 500 — 1 configured error.
	assert.False(t, recordDisable(1, 500, true))
	// Channel 2, same status — different key, should start fresh.
	assert.False(t, recordDisable(2, 500, true))
	// Channel 1 — 2nd error, should trigger.
	assert.True(t, recordDisable(1, 500, true))
	// Channel 2 — 2nd error, should also trigger.
	assert.True(t, recordDisable(2, 500, true))
}

func TestCheckAndRecordDisable_ThresholdOne(t *testing.T) {
	resetDisableWindowConfig()
	common.ConfiguredDisableThreshold = 1
	common.RedisEnabled = false
	resetDisableWindowLimiter()

	// threshold=1 means "one-strike" — the first error should immediately trigger.
	assert.True(t, recordDisable(1, 500, true), "threshold=1 should trigger on first error")
}

func TestCheckAndRecordDisable_ThresholdZero(t *testing.T) {
	resetDisableWindowConfig()
	common.ConfiguredDisableThreshold = 0
	common.RedisEnabled = false
	resetDisableWindowLimiter()

	// threshold=0 is a safety guard — should never trigger.
	assert.False(t, recordDisable(1, 500, true), "threshold=0 should never trigger disable")
}

func TestCheckAndRecordDisable_ConfiguredAndUnconfiguredIndependent(t *testing.T) {
	resetDisableWindowConfig()
	common.RedisEnabled = false
	resetDisableWindowLimiter()

	// Channel 1, status 500, configured tier — 1 error.
	assert.False(t, recordDisable(1, 500, true))
	// Channel 1, status 500, unconfigured tier — different key, starts fresh.
	assert.False(t, recordDisable(1, 500, false))
	// Configured tier — 2nd error, should trigger.
	assert.True(t, recordDisable(1, 500, true))
	// Unconfigured tier — still only 1 error, should not trigger.
	assert.False(t, recordDisable(1, 500, false))
}

func TestShouldDisableChannelWithDecision_ConfiguredError(t *testing.T) {
	resetDisableWindowConfig()
	common.AutomaticDisableChannelEnabled = true
	common.RedisEnabled = false
	resetDisableWindowLimiter()

	// Build a minimal NewAPIError with a status code in the configured range.
	// Default AutomaticDisableStatusCodeRanges includes 401.
	err := types.NewOpenAIError(
		fmt.Errorf("unauthorized"),
		types.ErrorCodeBadResponseStatusCode,
		401,
	)

	// First call: should not disable.
	decision := ShouldDisableChannelWithDecision(10, err)
	require.False(t, decision.ShouldDisable)
	require.Empty(t, decision.Reason)

	// Second call: should disable with a reason carrying window detail.
	decision = ShouldDisableChannelWithDecision(10, err)
	require.True(t, decision.ShouldDisable)
	require.NotEmpty(t, decision.Reason)
	assert.Contains(t, decision.Reason, "401")
	assert.Contains(t, decision.Reason, "failures in", "configured reason must carry window detail")
}

func TestShouldDisableChannelWithDecision_Disabled(t *testing.T) {
	resetDisableWindowConfig()
	common.AutomaticDisableChannelEnabled = false
	common.RedisEnabled = false
	resetDisableWindowLimiter()

	err := types.NewOpenAIError(
		fmt.Errorf("error"),
		types.ErrorCodeBadResponseStatusCode,
		500,
	)

	decision := ShouldDisableChannelWithDecision(10, err)
	require.False(t, decision.ShouldDisable)
}

func TestShouldDisableChannelWithDecision_NilError(t *testing.T) {
	resetDisableWindowConfig()
	common.AutomaticDisableChannelEnabled = true
	common.RedisEnabled = false

	decision := ShouldDisableChannelWithDecision(10, nil)
	require.False(t, decision.ShouldDisable)
}
