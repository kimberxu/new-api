package channelslowstream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// 渠道流式速率降级（Channel Slow-Stream Demotion）
//
// 利用成功流式请求的 generation TPS（outputTokens / generationMs），
// 按 (channelId, model) 维度统计连续慢速事件，达到阈值后临时将该渠道
// 在该模型上的优先级拍平到 DemotedPriority，到期自动恢复。
// 只降 priority，不动 weight；只有一档降级，无阶梯。

// demotionState 内存模式下的单个 (channelId, model) 降级记录。
type demotionState struct {
	mu           sync.Mutex
	demotedUntil int64 // unix timestamp，到期恢复
	count        int   // 当前窗口内连续慢速计数
	lastSlowAt   int64 // 上次慢速事件时间戳
}

var slowStreamMap sync.Map // key: fmt.Sprintf("%d:%s", channelId, model) -> *demotionState

// Redis key 命名空间
const slowStreamRedisNamespace = "slowStream"

func slowStreamRedisWindowKey(channelId int, model string) string {
	return fmt.Sprintf("%s:%d:%s", slowStreamRedisNamespace, channelId, model)
}

func slowStreamRedisDemotedKey(channelId int, model string) string {
	return fmt.Sprintf("%s:demoted:%d:%s", slowStreamRedisNamespace, channelId, model)
}

// slowStreamLuaScript atomically pushes a timestamp, trims to the threshold,
// sets expiry, and returns the current count. Same pattern as
// channelDisableWindowLuaScript.
const slowStreamLuaScript = `
local count = redis.call('LPUSH', KEYS[1], ARGV[1])
if count > tonumber(ARGV[2]) then
  redis.call('LTRIM', KEYS[1], 0, tonumber(ARGV[2]) - 1)
  count = tonumber(ARGV[2])
end
redis.call('EXPIRE', KEYS[1], ARGV[3])
return count
`

var slowStreamLuaSha string

func getSlowStreamLuaSha() string {
	if slowStreamLuaSha != "" {
		return slowStreamLuaSha
	}
	ctx := context.Background()
	sha, err := common.RDB.ScriptLoad(ctx, slowStreamLuaScript).Result()
	if err != nil {
		return ""
	}
	slowStreamLuaSha = sha
	return sha
}

// isExcludedChannel 判断渠道是否在慢流式降级的排除列表中。
func isExcludedChannel(channelId int, setting *operation_setting.ChannelSlowStreamSetting) bool {
	for _, id := range setting.ExcludeChannelIDs {
		if id == channelId {
			return true
		}
	}
	return false
}

// RecordSlowStream 记录一次慢速流式请求事件，返回 true 表示本次触发降级。
// 已处于降级态时不重复降级，仅续期 demotedUntil。
// 配置未启用、渠道在排除列表、tps 未低于阈值、Redis 出错时均返回 false（fail-open）。
func RecordSlowStream(channelId int, model string, tps float64) bool {
	setting := operation_setting.GetChannelSlowStreamSetting()
	if !setting.Enabled || setting.Threshold <= 0 {
		return false
	}
	if isExcludedChannel(channelId, &setting) {
		return false
	}
	if tps >= setting.MinTps {
		// 不慢：重置窗口计数（清零 count，保留 demotedUntil——
		// 一次快请求不得取消进行中的定时降级；降级到期由
		// GetDemotedPriority / CleanupExpiredDemotions 处理）
		if common.RedisEnabled && common.RDB != nil {
			ctx := context.Background()
			common.RDB.Del(ctx, slowStreamRedisWindowKey(channelId, model))
		} else {
			if value, ok := slowStreamMap.Load(memoryKey(channelId, model)); ok {
				state := value.(*demotionState)
				state.mu.Lock()
				now := time.Now().Unix()
				if state.demotedUntil <= now {
					// 未降级或降级已到期：直接移除条目
					slowStreamMap.Delete(memoryKey(channelId, model))
				} else {
					// 降级中：只重置窗口计数，保留 demotedUntil
					state.count = 0
					state.lastSlowAt = 0
				}
				state.mu.Unlock()
			}
		}
		return false
	}
	// 慢速事件
	if common.RedisEnabled && common.RDB != nil {
		return redisRecordSlow(channelId, model, &setting)
	}
	return memoryRecordSlow(channelId, model, &setting)
}

func memoryKey(channelId int, model string) string {
	return fmt.Sprintf("%d:%s", channelId, model)
}

func memoryRecordSlow(channelId int, model string, setting *operation_setting.ChannelSlowStreamSetting) bool {
	now := time.Now().Unix()
	key := memoryKey(channelId, model)
	value, _ := slowStreamMap.LoadOrStore(key, &demotionState{})
	state := value.(*demotionState)
	state.mu.Lock()
	defer state.mu.Unlock()
	if now-state.lastSlowAt > setting.WindowSeconds {
		// 窗口过期，重新计数
		state.count = 0
	}
	state.count++
	state.lastSlowAt = now
	if state.count < setting.Threshold {
		return false
	}
	if state.demotedUntil > now {
		// 已降级：仅续期
		state.demotedUntil = now + setting.DemoteDurationSec
		return false
	}
	state.demotedUntil = now + setting.DemoteDurationSec
	return true
}

func redisRecordSlow(channelId int, model string, setting *operation_setting.ChannelSlowStreamSetting) bool {
	ctx := context.Background()
	key := slowStreamRedisWindowKey(channelId, model)
	now := time.Now().Unix()

	var count int64
	var err error
	sha := getSlowStreamLuaSha()
	if sha != "" {
		count, err = common.RDB.EvalSha(ctx, sha, []string{key}, now, setting.Threshold, setting.WindowSeconds).Int64()
	} else {
		count, err = common.RDB.Eval(ctx, slowStreamLuaScript, []string{key}, now, setting.Threshold, setting.WindowSeconds).Int64()
	}
	if err != nil {
		// fail-open：Redis 出错不影响请求
		return false
	}
	if count < int64(setting.Threshold) {
		return false
	}

	demotedKey := slowStreamRedisDemotedKey(channelId, model)
	demotedUntil := now + setting.DemoteDurationSec
	ttl := time.Duration(setting.DemoteDurationSec) * time.Second
	existing, getErr := common.RDB.Get(ctx, demotedKey).Int64()
	if getErr == nil && existing > now {
		// 已降级：仅续期
		if err := common.RDB.Set(ctx, demotedKey, demotedUntil, ttl).Err(); err != nil {
			return false
		}
		return false
	}
	if err := common.RDB.Set(ctx, demotedKey, demotedUntil, ttl).Err(); err != nil {
		return false
	}
	return true
}

// GetDemotedPriority 返回渠道在指定 model 上的降级后优先级。
// 未降级、已过期或 Redis 出错时返回 originalPriority（demoted=false）。
// 降级中返回 min(originalPriority, DemotedPriority)。
func GetDemotedPriority(channelId int, model string, originalPriority int64) (bool, int64) {
	setting := operation_setting.GetChannelSlowStreamSetting()
	if !setting.Enabled {
		return false, originalPriority
	}
	if isExcludedChannel(channelId, &setting) {
		// 排除渠道不参与降级：即使有历史降级记录也直接返回原优先级
		return false, originalPriority
	}
	now := time.Now().Unix()
	var demotedUntil int64
	if common.RedisEnabled && common.RDB != nil {
		v, err := common.RDB.Get(context.Background(), slowStreamRedisDemotedKey(channelId, model)).Int64()
		if err != nil {
			// fail-open
			return false, originalPriority
		}
		demotedUntil = v
	} else {
		key := memoryKey(channelId, model)
		value, ok := slowStreamMap.Load(key)
		if !ok {
			return false, originalPriority
		}
		state := value.(*demotionState)
		state.mu.Lock()
		demotedUntil = state.demotedUntil
		state.mu.Unlock()
	}
	if demotedUntil <= now {
		return false, originalPriority
	}
	priority := originalPriority
	if setting.DemotedPriority < priority {
		priority = setting.DemotedPriority
	}
	return true, priority
}

var cleanupOnce sync.Once

// Init 启动后台恢复 goroutine，定期清理过期的降级记录。
func Init() {
	cleanupOnce.Do(func() {
		go func() {
			for {
				time.Sleep(60 * time.Second)
				CleanupExpiredDemotions()
			}
		}()
	})
}

// CleanupExpiredDemotions 清除已降级且到期的记录。
// 内存模式：遍历删除 demotedUntil 已过期的条目（恢复后重新计数）；
// 未降级条目的窗口计数保留（demotedUntil == 0），否则后台清理会清掉
// 窗口内慢速计数，导致阈值永远无法达到。
// Redis 模式：依赖 key 的 EXPIRE 自动过期，无需主动清理。
func CleanupExpiredDemotions() {
	if common.RedisEnabled && common.RDB != nil {
		return
	}
	now := time.Now().Unix()
	slowStreamMap.Range(func(key, value any) bool {
		state := value.(*demotionState)
		state.mu.Lock()
		expired := state.demotedUntil > 0 && state.demotedUntil <= now
		state.mu.Unlock()
		if expired {
			slowStreamMap.Delete(key)
		}
		return true
	})
}
