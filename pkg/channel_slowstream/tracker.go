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
// results 保存最近 sampleSize 次采样结果（true=慢，false=快），
// slowCount 为其中慢事件数；快事件只挤掉最旧样本，不洗白历史慢记录。
type demotionState struct {
	mu           sync.Mutex
	demotedUntil int64   // unix timestamp，到期恢复
	results      []bool  // ring buffer，append 端为最新样本
	slowCount    int     // 当前窗口内慢事件数
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

// slowStreamLuaScript ring buffer 语义：LPUSH 本次采样结果（1=慢 0=快），
// LTRIM 保留最近 sampleSize 条，返回窗口内慢事件总数。
const slowStreamLuaScript = `
redis.call('LPUSH', KEYS[1], ARGV[1])
redis.call('LTRIM', KEYS[1], 0, tonumber(ARGV[2]) - 1)
local slowCount = 0
local items = redis.call('LRANGE', KEYS[1], 0, -1)
for _, v in ipairs(items) do
  if tonumber(v) == 1 then
    slowCount = slowCount + 1
  end
end
return slowCount
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
	sampleSize        int   // ring buffer 容量（最近多少次采样）
	threshold         int   // 窗口内慢事件次数触发降级
	demoteDurationSec int64 // 降级持续时间秒
}

// sanitize 修正无效配置：ring buffer 容量至少等于 threshold，
// 否则窗口内永远凑不够慢事件数，降级永不触发。
func (w *demotionWindow) sanitize() {
	if w.sampleSize < w.threshold {
		w.sampleSize = w.threshold
	}
	if w.threshold <= 0 {
		w.threshold = 1
	}
	if w.demoteDurationSec <= 0 {
		w.demoteDurationSec = 1
	}
}

func slowStreamRedisTtftWindowKey(channelId int, model string) string {
	return fmt.Sprintf("%s:ttftWindow:%d:%s", slowStreamRedisNamespace, channelId, model)
}

func slowStreamRedisTtftDemotedKey(channelId int, model string) string {
	return fmt.Sprintf("%s:ttftDemoted:%d:%s", slowStreamRedisNamespace, channelId, model)
}

// RecordSlowStream 记录一次流式请求的生成速率采样，返回 true 表示本次触发降级。
// 快慢都在 ring buffer 中记录；慢事件数达到 threshold 触发降级。
// 已处于降级态时不重复降级，仅续期 demotedUntil。
// 配置未启用、渠道在排除列表、Redis 出错时均返回 false（fail-open）。
func RecordSlowStream(channelId int, model string, tps float64) bool {
	setting := operation_setting.GetChannelSlowStreamSetting()
	if !setting.Enabled || setting.Threshold <= 0 {
		return false
	}
	if isExcludedChannel(channelId, &setting) {
		return false
	}
	window := demotionWindow{setting.SampleSize, setting.Threshold, setting.DemoteDurationSec}
	window.sanitize()
	isSlow := tps < setting.MinTps
	if common.RedisEnabled && common.RDB != nil {
		return redisRecordSlow(slowStreamRedisWindowKey(channelId, model), slowStreamRedisDemotedKey(channelId, model), window, isSlow)
	}
	return memoryRecordSlow(&slowStreamMap, memoryKey(channelId, model), window, isSlow)
}

// RecordSlowTtft 记录一次首字延迟（TTFT）采样，返回 true 表示本次触发降级。
// frtMs 为首字延迟毫秒数；超过 MaxTtftMs 计为慢事件。
// 快慢都在 ring buffer 中记录；慢事件数达到 threshold 触发降级。
// 配置未启用、渠道在排除列表、Redis 出错时均返回 false（fail-open）。
func RecordSlowTtft(channelId int, model string, frtMs int64) bool {
	setting := operation_setting.GetChannelSlowStreamSetting()
	if !setting.TtftEnabled || setting.TtftThreshold <= 0 {
		return false
	}
	if isExcludedChannel(channelId, &setting) {
		return false
	}
	window := demotionWindow{setting.TtftSampleSize, setting.TtftThreshold, setting.DemoteDurationSec}
	window.sanitize()
	isSlow := frtMs > setting.MaxTtftMs
	if common.RedisEnabled && common.RDB != nil {
		return redisRecordSlow(slowStreamRedisTtftWindowKey(channelId, model), slowStreamRedisTtftDemotedKey(channelId, model), window, isSlow)
	}
	return memoryRecordSlow(&slowStreamTtftMap, memoryKey(channelId, model), window, isSlow)
}

func memoryKey(channelId int, model string) string {
	return fmt.Sprintf("%d:%s", channelId, model)
}

// memoryRecordSlow 内存模式 ring buffer 计数：采样结果（快/慢）入队，
// 超出 sampleSize 时弹出最旧样本并同步 slowCount；slowCount 达到 threshold
// 触发降级。不按时间过期——长请求的采样延迟不影响窗口判定。
func memoryRecordSlow(windowMap *sync.Map, key string, window demotionWindow, isSlow bool) bool {
	now := time.Now().Unix()
	value, _ := windowMap.LoadOrStore(key, &demotionState{})
	state := value.(*demotionState)
	state.mu.Lock()
	defer state.mu.Unlock()
	state.results = append(state.results, isSlow)
	if isSlow {
		state.slowCount++
	}
	if len(state.results) > window.sampleSize {
		if state.results[0] {
			state.slowCount--
		}
		state.results = state.results[1:]
	}
	if state.slowCount < window.threshold {
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

func redisRecordSlow(windowKey string, demotedKey string, window demotionWindow, isSlow bool) bool {
	ctx := context.Background()
	now := time.Now().Unix()

	slowMark := "0"
	if isSlow {
		slowMark = "1"
	}
	var slowCount int64
	var err error
	sha := getSlowStreamLuaSha()
	if sha != "" {
		slowCount, err = common.RDB.EvalSha(ctx, sha, []string{windowKey}, slowMark, window.sampleSize).Int64()
	} else {
		slowCount, err = common.RDB.Eval(ctx, slowStreamLuaScript, []string{windowKey}, slowMark, window.sampleSize).Int64()
	}
	if err != nil {
		// fail-open：Redis 出错不影响请求
		return false
	}
	if slowCount < int64(window.threshold) {
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
