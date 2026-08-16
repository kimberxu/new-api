package service

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	channelRateLimitRedisNamespace = "channelRateLimit:v2"
)

var (
	channelRateLimitMemoryLimiterOnce sync.Once
	channelRateLimitMemoryLimiter     *common.InMemoryRateLimiter
)

func getChannelRateLimitMemoryLimiter() *common.InMemoryRateLimiter {
	channelRateLimitMemoryLimiterOnce.Do(func() {
		l := &common.InMemoryRateLimiter{}
		l.Init(10 * time.Minute)
		channelRateLimitMemoryLimiter = l
	})
	return channelRateLimitMemoryLimiter
}

func channelRateLimitRedisKey(channelID int, metric string) string {
	return fmt.Sprintf("%s:%d:%s", channelRateLimitRedisNamespace, channelID, metric)
}

// rpmToLimitWindow converts an RPM (float, can be fractional) into integer
// (limit, windowSeconds) for fixed-window rate limiting.
//
//   - limit = max(1, ceil(RPM))
//   - window = ceil(60 × limit / RPM)
//
// This ensures the effective rate matches the configured RPM regardless of
// whether it's an integer or a decimal like 0.5 or 2.5.
func rpmToLimitWindow(rpm float64) (limit int, windowSeconds int64) {
	if rpm <= 0 {
		return 0, 60
	}
	limit = int(math.Ceil(rpm))
	if limit < 1 {
		limit = 1
	}
	windowSeconds = int64(math.Ceil(60.0 * float64(limit) / rpm))
	if windowSeconds < 1 {
		windowSeconds = 1
	}
	return
}

// CheckChannelRateLimit checks if a channel has exceeded its RPM limit.
// Returns true if the request is allowed (not rate limited).
func CheckChannelRateLimit(channelID int) bool {
	if channelID <= 0 {
		return true
	}

	globalSetting := operation_setting.GetChannelRateLimitSetting()
	if globalSetting == nil || !globalSetting.Enabled {
		return true
	}

	channel, err := model.CacheGetChannel(channelID)
	if err != nil || channel == nil {
		return true
	}

	otherSettings := channel.GetOtherSettings()
	if !otherSettings.RateLimitEnabled {
		return true
	}

	rpm := otherSettings.RateLimitRPM
	if rpm <= 0 {
		rpm = globalSetting.DefaultRPM
	}

	limit, window := rpmToLimitWindow(rpm)

	if common.RedisEnabled && common.RDB != nil {
		if !channelRateLimitRedisTake(channelID, "rpm", limit, window) {
			return false
		}
	} else {
		key := channelRateLimitRedisKey(channelID, "rpm")
		if !getChannelRateLimitMemoryLimiter().Request(key, limit, window) {
			return false
		}
	}

	return true
}

// channelRateLimitLuaScript atomically increments the counter and sets TTL on
// the first request, avoiding the race where a crash between INCR and EXPIRE
// leaves a key without a TTL (permanent rate-limit lockout).
const channelRateLimitLuaScript = `
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
`

var channelRateLimitLuaSha string

func getChannelRateLimitLuaSha() string {
	if channelRateLimitLuaSha != "" {
		return channelRateLimitLuaSha
	}
	ctx := context.Background()
	sha, err := common.RDB.ScriptLoad(ctx, channelRateLimitLuaScript).Result()
	if err != nil {
		// Fallback: return empty so callers use Eval directly.
		return ""
	}
	channelRateLimitLuaSha = sha
	return sha
}

func channelRateLimitRedisTake(channelID int, metric string, limit int, windowSec int64) bool {
	ctx := context.Background()
	key := channelRateLimitRedisKey(channelID, metric)

	var count int64
	var err error
	sha := getChannelRateLimitLuaSha()
	if sha != "" {
		count, err = common.RDB.EvalSha(ctx, sha, []string{key}, windowSec).Int64()
	} else {
		count, err = common.RDB.Eval(ctx, channelRateLimitLuaScript, []string{key}, windowSec).Int64()
	}
	if err != nil {
		return true // allow on error
	}
	if count > int64(limit) {
		return false
	}
	return true
}

// IsChannelRateLimited reports whether a channel is currently rate-limited
// without consuming a slot. Use this for read-only checks (e.g. computing
// Retry-After for a 429 response) where the caller must NOT increment the
// counter.
func IsChannelRateLimited(channelID int) bool {
	if channelID <= 0 {
		return false
	}
	globalSetting := operation_setting.GetChannelRateLimitSetting()
	if globalSetting == nil || !globalSetting.Enabled {
		return false
	}
	channel, err := model.CacheGetChannel(channelID)
	if err != nil || channel == nil {
		return false
	}
	otherSettings := channel.GetOtherSettings()
	if !otherSettings.RateLimitEnabled {
		return false
	}
	rpm := otherSettings.RateLimitRPM
	if rpm <= 0 {
		rpm = globalSetting.DefaultRPM
	}
	limit, _ := rpmToLimitWindow(rpm)

	if common.RedisEnabled && common.RDB != nil {
		ctx := context.Background()
		key := channelRateLimitRedisKey(channelID, "rpm")
		count, err := common.RDB.Get(ctx, key).Int64()
		if err != nil {
			return false // not rate-limited on error
		}
		return count > int64(limit)
	}
	// In-memory limiter does not expose a read-only peek; conservatively
	// report not-limited so the 429 path is driven by actual CheckChannelRateLimit.
	return false
}

// GetChannelRPM returns the RPM configured for a channel, considering both
// per-channel setting and global default. Returns 0 if rate limiting is disabled.
func GetChannelRPM(channelID int) float64 {
	if channelID <= 0 {
		return 0
	}

	globalSetting := operation_setting.GetChannelRateLimitSetting()
	if globalSetting == nil || !globalSetting.Enabled {
		return 0
	}

	channel, err := model.CacheGetChannel(channelID)
	if err != nil || channel == nil {
		return 0
	}

	otherSettings := channel.GetOtherSettings()
	if !otherSettings.RateLimitEnabled {
		return 0
	}

	rpm := otherSettings.RateLimitRPM
	if rpm <= 0 {
		rpm = globalSetting.DefaultRPM
	}
	return rpm
}

// HasRateLimitedChannelsForModel checks whether ANY channel in the given
// group+model has an active rate limit (hit the limit). Returns the minimum
// retry-after seconds across all rate-limited channels.
func HasRateLimitedChannelsForModel(group string, modelName string) (bool, int64) {
	if group == "" || modelName == "" {
		return false, 0
	}

	channelIDs := model.GetChannelIDsForGroupModel(group, modelName)
	if len(channelIDs) == 0 {
		return false, 0
	}

	rateLimitHit := false
	var minRetryAfter int64 = 0

	for _, cid := range channelIDs {
		rpm := GetChannelRPM(cid)
		if rpm <= 0 {
			continue
		}

		if IsChannelRateLimited(cid) {
			rateLimitHit = true
			_, window := rpmToLimitWindow(rpm)
			if minRetryAfter == 0 || window < minRetryAfter {
				minRetryAfter = window
			}
		}
	}

	return rateLimitHit, minRetryAfter
}
