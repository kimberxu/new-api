package channelslowstream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// 渠道流式速率与首字延迟降级（Channel Slow-Stream & TTFT Demotion）
//
// 两个独立降级源：
//   - 生成速率（TPS）：outputTokens / generationMs，持续低于 MinTps 计为慢事件
//   - 首字延迟（TTFT）：FirstResponseTime - StartTime，超过 MaxTtftMs 计为慢事件
//
// 两源各自维护滑动窗口与降级标记，互不干扰；任一触发即降级。
// 按 (channelId, model) 维度统计，达到阈值后临时将该渠道在对应模型上的
// 优先级拍平到 DemotedPriority，到期自动恢复。
// 只降 priority，不动 weight；只有一档降级，无阶梯。

// demotionState 内存模式下的单个 (channelId, model) 降级记录。
type demotionState struct {
	mu           sync.Mutex
	demotedUntil int64 // unix timestamp，到期恢复
	count        int   // 当前窗口内连续慢速计数
	lastSlowAt   int64 // 上次慢速事件时间戳
}

var slowStreamMap sync.Map     // 生成速率降级：key: fmt.Sprintf("%d:%s", channelId, model) -> *demotionState
var slowStreamTtftMap sync.Map // 首字延迟降级：key 同上

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

// demotionWindow 一次降级事件源（生成慢速或首字延迟）的窗口配置。
type demotionWindow struct {
	windowSeconds     int64
	threshold         int
	demoteDurationSec int64
}

func slowStreamRedisTtftWindowKey(channelId int, model string) string {
	return fmt.Sprintf("%s:ttftWindow:%d:%s", slowStreamRedisNamespace, channelId, model)
}

func slowStreamRedisTtftDemotedKey(channelId int, model string) string {
	return fmt.Sprintf("%s:ttftDemoted:%d:%s", slowStreamRedisNamespace, channelId, model)
}

// resetSlowWindow 重置一个事件源（生成慢速或首字延迟）的窗口计数。
// 清零 count，保留 demotedUntil——一次正常请求不得取消进行中的定时降级；
// 降级到期由 GetDemotedPriority / CleanupExpiredDemotions 处理。
// memoryKey 为内存 map 的 key，redisWindowKey 为 Redis 窗口 key（两者格式不同）。
func resetSlowWindow(windowMap *sync.Map, memoryKey string, redisWindowKey string) {
	if common.RedisEnabled && common.RDB != nil {
		ctx := context.Background()
		common.RDB.Del(ctx, redisWindowKey)
		return
	}
	if value, ok := windowMap.Load(memoryKey); ok {
		state := value.(*demotionState)
		state.mu.Lock()
		now := time.Now().Unix()
		if state.demotedUntil <= now {
			// 未降级或降级已到期：直接移除条目
			windowMap.Delete(memoryKey)
		} else {
			// 降级中：只重置窗口计数，保留 demotedUntil
			state.count = 0
			state.lastSlowAt = 0
		}
		state.mu.Unlock()
	}
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
		// 不慢：重置窗口计数
		resetSlowWindow(&slowStreamMap, memoryKey(channelId, model), slowStreamRedisWindowKey(channelId, model))
		return false
	}
	// 慢速事件
	window := demotionWindow{setting.WindowSeconds, setting.Threshold, setting.DemoteDurationSec}
	if common.RedisEnabled && common.RDB != nil {
		return redisRecordSlow(slowStreamRedisWindowKey(channelId, model), slowStreamRedisDemotedKey(channelId, model), window)
	}
	return memoryRecordSlow(&slowStreamMap, memoryKey(channelId, model), window)
}

// RecordSlowTtft 记录一次首字延迟（TTFT）慢速事件，返回 true 表示本次触发降级。
// frtMs 为首字延迟毫秒数；超过 MaxTtftMs 计为慢事件。
// 配置未启用、渠道在排除列表、frt 未超过阈值、Redis 出错时均返回 false（fail-open）。
func RecordSlowTtft(channelId int, model string, frtMs int64) bool {
	setting := operation_setting.GetChannelSlowStreamSetting()
	if !setting.TtftEnabled || setting.TtftThreshold <= 0 {
		return false
	}
	if isExcludedChannel(channelId, &setting) {
		return false
	}
	if frtMs <= setting.MaxTtftMs {
		// 首字不快：重置 TTFT 窗口计数
		resetSlowWindow(&slowStreamTtftMap, memoryKey(channelId, model), slowStreamRedisTtftWindowKey(channelId, model))
		return false
	}
	// 慢首字事件
	window := demotionWindow{setting.TtftWindowSeconds, setting.TtftThreshold, setting.DemoteDurationSec}
	if common.RedisEnabled && common.RDB != nil {
		return redisRecordSlow(slowStreamRedisTtftWindowKey(channelId, model), slowStreamRedisTtftDemotedKey(channelId, model), window)
	}
	return memoryRecordSlow(&slowStreamTtftMap, memoryKey(channelId, model), window)
}

func memoryKey(channelId int, model string) string {
	return fmt.Sprintf("%d:%s", channelId, model)
}

func memoryRecordSlow(windowMap *sync.Map, key string, window demotionWindow) bool {
	now := time.Now().Unix()
	value, _ := windowMap.LoadOrStore(key, &demotionState{})
	state := value.(*demotionState)
	state.mu.Lock()
	defer state.mu.Unlock()
	if now-state.lastSlowAt > window.windowSeconds {
		// 窗口过期，重新计数
		state.count = 0
	}
	state.count++
	state.lastSlowAt = now
	if state.count < window.threshold {
		return false
	}
	if state.demotedUntil > now {
		// 已降级：仅续期
		state.demotedUntil = now + window.demoteDurationSec
		return false
	}
	state.demotedUntil = now + window.demoteDurationSec
	return true
}

func redisRecordSlow(windowKey string, demotedKey string, window demotionWindow) bool {
	ctx := context.Background()
	now := time.Now().Unix()

	var count int64
	var err error
	sha := getSlowStreamLuaSha()
	if sha != "" {
		count, err = common.RDB.EvalSha(ctx, sha, []string{windowKey}, now, window.threshold, window.windowSeconds).Int64()
	} else {
		count, err = common.RDB.Eval(ctx, slowStreamLuaScript, []string{windowKey}, now, window.threshold, window.windowSeconds).Int64()
	}
	if err != nil {
		// fail-open：Redis 出错不影响请求
		return false
	}
	if count < int64(window.threshold) {
		return false
	}

	demotedUntil := now + window.demoteDurationSec
	ttl := time.Duration(window.demoteDurationSec) * time.Second
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
// 生成速率与首字延迟两个降级源任一生效即降级（取 min）。
// 未降级、已过期或 Redis 出错时返回 originalPriority（demoted=false）。
func GetDemotedPriority(channelId int, model string, originalPriority int64) (bool, int64) {
	setting := operation_setting.GetChannelSlowStreamSetting()
	if !setting.Enabled && !setting.TtftEnabled {
		return false, originalPriority
	}
	if isExcludedChannel(channelId, &setting) {
		// 排除渠道不参与降级：即使有历史降级记录也直接返回原优先级
		return false, originalPriority
	}
	now := time.Now().Unix()
	key := memoryKey(channelId, model)
	var demotedUntil int64

	// 生成速率降级源
	if setting.Enabled {
		if common.RedisEnabled && common.RDB != nil {
			if v, err := common.RDB.Get(context.Background(), slowStreamRedisDemotedKey(channelId, model)).Int64(); err == nil && v > demotedUntil {
				demotedUntil = v
			}
		} else if value, ok := slowStreamMap.Load(key); ok {
			state := value.(*demotionState)
			state.mu.Lock()
			if state.demotedUntil > demotedUntil {
				demotedUntil = state.demotedUntil
			}
			state.mu.Unlock()
		}
	}

	// 首字延迟降级源
	if setting.TtftEnabled {
		if common.RedisEnabled && common.RDB != nil {
			if v, err := common.RDB.Get(context.Background(), slowStreamRedisTtftDemotedKey(channelId, model)).Int64(); err == nil && v > demotedUntil {
				demotedUntil = v
			}
		} else if value, ok := slowStreamTtftMap.Load(key); ok {
			state := value.(*demotionState)
			state.mu.Lock()
			if state.demotedUntil > demotedUntil {
				demotedUntil = state.demotedUntil
			}
			state.mu.Unlock()
		}
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
	cleanupMap := func(m sync.Map) {
		m.Range(func(key, value any) bool {
			state := value.(*demotionState)
			state.mu.Lock()
			expired := state.demotedUntil > 0 && state.demotedUntil <= now
			state.mu.Unlock()
			if expired {
				m.Delete(key)
			}
			return true
		})
	}
	cleanupMap(slowStreamMap)
	cleanupMap(slowStreamTtftMap)
}
