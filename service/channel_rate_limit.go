package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	channelRateLimitRedisNamespace = "channelRateLimit:v2"
	channelRateLimitDuration       = 60 // 1 minute window
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

// CheckChannelRateLimit checks if a channel has exceeded its RPM limit.
// Returns true if the request is allowed (not rate limited).
// Uses a fixed-window rate limiting approach (INCR + EXPIRE for Redis,
// timestamp queue for in-memory mode).
func CheckChannelRateLimit(channelID int) bool {
	if channelID <= 0 {
		return true
	}

	setting := operation_setting.GetChannelRateLimitSetting()
	if setting == nil || !setting.Enabled {
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
		rpm = setting.DefaultRPM
	}

	if common.RedisEnabled && common.RDB != nil {
		if !channelRateLimitRedisTake(channelID, "rpm", rpm) {
			return false
		}
	} else {
		if !getChannelRateLimitMemoryLimiter().Request(channelRateLimitRedisKey(channelID, "rpm"), rpm, channelRateLimitDuration) {
			return false
		}
	}

	return true
}

func channelRateLimitRedisTake(channelID int, metric string, maxCount int) bool {
	ctx := context.Background()
	key := channelRateLimitRedisKey(channelID, metric)

	count, err := common.RDB.Incr(ctx, key).Result()
	if err != nil {
		return true // allow on error
	}
	if count == 1 {
		common.RDB.Expire(ctx, key, channelRateLimitDuration*time.Second)
	}
	if count > int64(maxCount) {
		return false
	}
	return true
}