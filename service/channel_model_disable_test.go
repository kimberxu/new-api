package service

import (
	"errors"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
)

func newModelLevelTestError(status int, msg string) *types.NewAPIError {
	return types.NewOpenAIError(errors.New(msg), types.ErrorCodeBadResponseStatusCode, status)
}

func TestIsModelLevelError(t *testing.T) {
	tests := []struct {
		name string
		err  *types.NewAPIError
		want bool
	}{
		// true: 404 with "model" in message (loose band)
		{"404 model not found", newModelLevelTestError(404, "model not found"), true},
		{"404 Model Not Found case-insensitive", newModelLevelTestError(404, "Model Not Found: gpt-4"), true},
		{"404 the model does not exist", newModelLevelTestError(404, "the model does not exist"), true},
		{"404 no model", newModelLevelTestError(404, "no model 'x' available"), true},
		// true: 400/422 keyword match
		{"400 invalid model", newModelLevelTestError(400, "invalid model id"), true},
		{"422 unsupported model", newModelLevelTestError(422, "unsupported model: foo"), true},
		{"400 model not supported", newModelLevelTestError(400, "model is not supported"), true},
		{"400 chinese model missing", newModelLevelTestError(400, "模型不存在"), true},
		{"400 chinese unknown model", newModelLevelTestError(400, "未知模型"), true},
		{"400 chinese unsupported model", newModelLevelTestError(400, "不支持的模型"), true},
		// false: non-404/400/422 status codes
		{"500 model not found", newModelLevelTestError(500, "model not found"), false},
		{"429 rate limited", newModelLevelTestError(429, "rate limited"), false},
		{"502 model not found", newModelLevelTestError(502, "model not found"), false},
		{"401 model not found", newModelLevelTestError(401, "model not found"), false},
		// false: 400/422 without keyword
		{"400 invalid json", newModelLevelTestError(400, "invalid json"), false},
		{"422 bad request body", newModelLevelTestError(422, "bad request body"), false},
		// false: 404 without the word model
		{"404 route not found", newModelLevelTestError(404, "route not found"), false},
		{"404 api not found", newModelLevelTestError(404, "api endpoint not found"), false},
		// nil
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsModelLevelError(tt.err))
		})
	}
}

// resetChannelModelDisableWindowLimiter replaces the singleton
// InMemoryRateLimiter so each test starts with a clean store.
func resetChannelModelDisableWindowLimiter() {
	channelModelDisableWindowMemoryLimiterOnce = sync.Once{}
	channelModelDisableWindowMemoryLimiter = nil
	_ = getChannelModelDisableWindowMemoryLimiter()
}

// recordDisableModel wraps CheckAndRecordDisableModel for boolean assertions.
func recordDisableModel(channelID int, modelName string, statusCode int, isConfiguredError bool) bool {
	triggered, _ := CheckAndRecordDisableModel(channelID, modelName, statusCode, isConfiguredError)
	return triggered
}

func TestCheckAndRecordDisableModel_ConfiguredThreshold(t *testing.T) {
	common.RedisEnabled = false
	common.ConfiguredDisableThreshold = 2
	common.ConfiguredDisableWindowSeconds = 600
	resetChannelModelDisableWindowLimiter()

	// Default: configured threshold = 2.
	assert.False(t, recordDisableModel(1, "gpt-4", 404, true), "first error should not trigger")
	assert.True(t, recordDisableModel(1, "gpt-4", 404, true), "second error should trigger")
}

func TestCheckAndRecordDisableModel_ModelsIndependent(t *testing.T) {
	common.RedisEnabled = false
	common.ConfiguredDisableThreshold = 2
	common.ConfiguredDisableWindowSeconds = 600
	resetChannelModelDisableWindowLimiter()

	// gpt-4: 1 error; gpt-4o: fresh key.
	assert.False(t, recordDisableModel(1, "gpt-4", 404, true))
	assert.False(t, recordDisableModel(1, "gpt-4o", 404, true), "gpt-4o first error must not trigger (key includes model)")
	// gpt-4: 2nd error triggers — proves the two models count independently.
	assert.True(t, recordDisableModel(1, "gpt-4", 404, true))
}

func TestCheckAndRecordDisableModel_DifferentStatusCodesIndependent(t *testing.T) {
	common.RedisEnabled = false
	common.ConfiguredDisableThreshold = 2
	common.ConfiguredDisableWindowSeconds = 600
	resetChannelModelDisableWindowLimiter()

	// Channel 1, gpt-4, status 404 — 1 configured error.
	assert.False(t, recordDisableModel(1, "gpt-4", 404, true))
	// Channel 1, gpt-4, status 400 — different key, fresh.
	assert.False(t, recordDisableModel(1, "gpt-4", 400, true))
	// Channel 1, gpt-4, status 404 — 2nd error triggers.
	assert.True(t, recordDisableModel(1, "gpt-4", 404, true))
}

func TestCheckAndRecordDisableModel_DifferentChannelsIndependent(t *testing.T) {
	common.RedisEnabled = false
	common.ConfiguredDisableThreshold = 2
	common.ConfiguredDisableWindowSeconds = 600
	resetChannelModelDisableWindowLimiter()

	// Channel 1, gpt-4 — 1 error.
	assert.False(t, recordDisableModel(1, "gpt-4", 404, true))
	// Channel 2 — fresh key.
	assert.False(t, recordDisableModel(2, "gpt-4", 404, true))
	// Channel 1 — 2nd error triggers.
	assert.True(t, recordDisableModel(1, "gpt-4", 404, true))
}

func TestCheckAndRecordDisableModel_ThresholdOne(t *testing.T) {
	common.RedisEnabled = false
	common.ConfiguredDisableThreshold = 1
	common.ConfiguredDisableWindowSeconds = 600
	resetChannelModelDisableWindowLimiter()

	assert.True(t, recordDisableModel(1, "gpt-4", 404, true), "threshold=1 triggers immediately")
}

func TestCheckAndRecordDisableModel_ThresholdZero(t *testing.T) {
	common.RedisEnabled = false
	common.ConfiguredDisableThreshold = 0
	common.ConfiguredDisableWindowSeconds = 600
	resetChannelModelDisableWindowLimiter()

	assert.False(t, recordDisableModel(1, "gpt-4", 404, true), "threshold=0 never disables")
}