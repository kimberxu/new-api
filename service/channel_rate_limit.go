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

func channelRateLimitRedisTake(channelID int, metric string, limit int, windowSec int64) bool {
	ctx := context.Background()
	key := channelRateLimitRedisKey(channelID, metric)

	count, err := common.RDB.Incr(ctx, key).Result()
	if err != nil {
		return true // allow on error
	}
	if count == 1 {
		common.RDB.Expire(ctx, key, time.Duration(windowSec)*time.Second)
	}
	if count > int64(limit) {
		return false
	}
	return true
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

		if !CheckChannelRateLimit(cid) {
			rateLimitHit = true
			_, window := rpmToLimitWindow(rpm)
			if minRetryAfter == 0 || window < minRetryAfter {
				minRetryAfter = window
			}
		}
	}

	return rateLimitHit, minRetryAfter
}
