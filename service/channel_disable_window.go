package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const channelDisableWindowRedisNamespace = "channelDisableWindow"

var (
	channelDisableWindowMemoryLimiterOnce sync.Once
	channelDisableWindowMemoryLimiter     *common.InMemoryRateLimiter
)

func getChannelDisableWindowMemoryLimiter() *common.InMemoryRateLimiter {
	channelDisableWindowMemoryLimiterOnce.Do(func() {
		l := &common.InMemoryRateLimiter{}
		l.Init(10 * time.Minute)
		channelDisableWindowMemoryLimiter = l
	})
	return channelDisableWindowMemoryLimiter
}

func channelDisableWindowRedisKey(channelID int, statusCode int, tier string) string {
	return fmt.Sprintf("%s:%d:%d:%s", channelDisableWindowRedisNamespace, channelID, statusCode, tier)
}

// channelDisableWindowLuaScript atomically pushes a timestamp, trims to the
// threshold, sets expiry, and returns the current count.
const channelDisableWindowLuaScript = `
local count = redis.call('LPUSH', KEYS[1], ARGV[1])
if count > tonumber(ARGV[2]) then
  redis.call('LTRIM', KEYS[1], 0, tonumber(ARGV[2]) - 1)
  count = tonumber(ARGV[2])
end
redis.call('EXPIRE', KEYS[1], ARGV[3])
return count
`

var channelDisableWindowLuaSha string

func getChannelDisableWindowLuaSha() string {
	if channelDisableWindowLuaSha != "" {
		return channelDisableWindowLuaSha
	}
	ctx := context.Background()
	sha, err := common.RDB.ScriptLoad(ctx, channelDisableWindowLuaScript).Result()
	if err != nil {
		return ""
	}
	channelDisableWindowLuaSha = sha
	return sha
}

func channelDisableWindowRedisTake(channelID int, statusCode int, tier string, threshold int, windowSec int64) bool {
	ctx := context.Background()
	key := channelDisableWindowRedisKey(channelID, statusCode, tier)
	now := time.Now().Unix()

	var count int64
	var err error
	sha := getChannelDisableWindowLuaSha()
	if sha != "" {
		count, err = common.RDB.EvalSha(ctx, sha, []string{key}, now, threshold, windowSec).Int64()
	} else {
		count, err = common.RDB.Eval(ctx, channelDisableWindowLuaScript, []string{key}, now, threshold, windowSec).Int64()
	}
	if err != nil {
		// Allow on error — same fail-open policy as channel rate limiting.
		return false
	}
	return count >= int64(threshold)
}

// CheckAndRecordDisable records one channel error and returns true if the
// sliding window threshold has been reached (i.e. the channel should be
// disabled). tier selects which window parameters apply:
//
//   - isConfiguredError=true  → strict window (ConfiguredDisable*)
//   - isConfiguredError=false → lenient window (UnconfiguredDisable*)
//
// The error identity key is channelID:statusCode:tier, so different status
// codes and different tiers are counted independently.
func CheckAndRecordDisable(channelID int, statusCode int, isConfiguredError bool) bool {
	var threshold int
	var windowSec int64
	var tier string

	if isConfiguredError {
		threshold = common.ConfiguredDisableThreshold
		windowSec = common.ConfiguredDisableWindowSeconds
		tier = "configured"
	} else {
		threshold = common.UnconfiguredDisableThreshold
		windowSec = common.UnconfiguredDisableWindowSeconds
		tier = "unconfigured"
	}

	if threshold <= 0 {
		// Threshold of 0 means "never disable" — safety guard.
		return false
	}

	if common.RedisEnabled && common.RDB != nil {
		return channelDisableWindowRedisTake(channelID, statusCode, tier, threshold, windowSec)
	}

	// In-memory sliding window.
	//
	// InMemoryRateLimiter.Request(key, maxRequestNum, duration) allows
	// maxRequestNum requests within the window and returns false on the
	// (maxRequestNum+1)-th. To trigger on the threshold-th error we pass
	// threshold-1 as maxRequestNum.
	//
	// Edge case: threshold=1 (one-strike). maxRequestNum=0 would still
	// create the queue and return true on the first call, so we handle it
	// explicitly: the first error immediately triggers.
	if threshold == 1 {
		return true
	}

	key := channelDisableWindowRedisKey(channelID, statusCode, tier)
	return !getChannelDisableWindowMemoryLimiter().Request(key, threshold-1, windowSec)
}
