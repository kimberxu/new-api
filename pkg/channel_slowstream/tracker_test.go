package channelslowstream

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTest 将配置切到内存模式并返回注册的配置指针（可修改字段），
// 测试结束后恢复原配置与 tracker 状态。
func setupTest(t *testing.T, enabled bool) *operation_setting.ChannelSlowStreamSetting {
	t.Helper()

	origRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = origRedis })

	cfg := config.GlobalConfig.Get("channel_slow_stream_setting").(*operation_setting.ChannelSlowStreamSetting)
	orig := *cfg
	*cfg = operation_setting.ChannelSlowStreamSetting{
		Enabled:           enabled,
		MinTps:            5.0,
		WindowSeconds:     300,
		SampleSize:        5,
		Threshold:         3,
		MinOutputTokens:   50,
		DemoteDurationSec: 600,
		DemotedPriority:   0,
		TtftEnabled:       enabled,
		MaxTtftMs:         5000,
		TtftWindowSeconds: 300,
		TtftSampleSize:    5,
		TtftThreshold:     3,
	}
	t.Cleanup(func() {
		*cfg = orig
		slowStreamMap = sync.Map{}
		slowStreamTtftMap = sync.Map{}
	})
	return cfg
}

func loadState(t *testing.T, channelId int, model string) *demotionState {
	t.Helper()
	value, ok := slowStreamMap.Load(memoryKey(channelId, model))
	if !ok {
		return nil
	}
	return value.(*demotionState)
}

func TestRecordSlowStream_Disabled_ReturnsFalse(t *testing.T) {
	setupTest(t, false)
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 0.1))
}

func TestRecordSlowStream_TriggersDemotionAfterThreshold(t *testing.T) {
	setupTest(t, true)
	// 前两次慢速不触发
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	// 第三次触发降级
	assert.True(t, RecordSlowStream(1, "gpt-4o", 1.0))
	demoted, p := GetDemotedPriority(1, "gpt-4o", 5)
	assert.True(t, demoted)
	assert.Equal(t, int64(0), p)
}

func TestRecordSlowStream_FastSampleSlidesWindow(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.SampleSize = 3
	cfg.Threshold = 2
	// ring buffer 核心语义：快事件不洗白历史慢记录，只挤掉最旧样本。
	// 慢 快 慢 → 窗口 [慢,快,慢]，慢=2 ≥ threshold=2 → 触发
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 10.0))
	assert.True(t, RecordSlowStream(1, "gpt-4o", 1.0))
	demoted, p := GetDemotedPriority(1, "gpt-4o", 5)
	assert.True(t, demoted)
	assert.Equal(t, int64(0), p)
}

func TestRecordSlowStream_FastSampleSlidesOutOldest(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.SampleSize = 3
	cfg.Threshold = 2
	// 慢 快 快 快：窗口 [快,快,快]，慢样本被挤出，slowCount 回到 0
	// （slowCount 从未达到 threshold=2，不触发降级）
	RecordSlowStream(1, "gpt-4o", 1.0)
	RecordSlowStream(1, "gpt-4o", 10.0)
	RecordSlowStream(1, "gpt-4o", 10.0)
	assert.False(t, RecordSlowStream(1, "gpt-4o", 10.0))
	state := loadState(t, 1, "gpt-4o")
	require.NotNil(t, state)
	assert.Equal(t, 0, state.slowCount)
	demoted, p := GetDemotedPriority(1, "gpt-4o", 5)
	assert.False(t, demoted)
	assert.Equal(t, int64(5), p)
	// 不同 (channelId, model) 独立计数
	assert.False(t, RecordSlowStream(1, "claude-3-5", 1.0))
	assert.False(t, RecordSlowStream(2, "gpt-4o", 1.0))
}

func TestRecordSlowStream_LongGapStillCounts(t *testing.T) {
	cfg := setupTest(t, true)
	// 废弃字段 window_seconds 即使设成 1 也不影响计数：
	// 固定数量窗口不按时间过期，长请求的采样延迟不会导致计数重置
	cfg.WindowSeconds = 1
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	// 间隔超过旧时间窗口后，慢事件仍连续计数，第三次直接触发降级
	time.Sleep(2100 * time.Millisecond)
	assert.True(t, RecordSlowStream(1, "gpt-4o", 1.0))
	demoted, p := GetDemotedPriority(1, "gpt-4o", 5)
	assert.True(t, demoted)
	assert.Equal(t, int64(0), p)
}

func TestRecordSlowStream_NoRedundantDemotion_RenewsUntil(t *testing.T) {
	setupTest(t, true)
	// 连续 3 次慢速触发降级
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.True(t, RecordSlowStream(1, "gpt-4o", 1.0))
	before := loadState(t, 1, "gpt-4o")
	require.NotNil(t, before)
	beforeUntil := before.demotedUntil
	// 跨秒后再慢速：不重复降级（false），仅续期 demotedUntil
	time.Sleep(1100 * time.Millisecond)
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	after := loadState(t, 1, "gpt-4o")
	require.NotNil(t, after)
	assert.Greater(t, after.demotedUntil, beforeUntil)
	// 降级状态持续
	demoted, p := GetDemotedPriority(1, "gpt-4o", 5)
	assert.True(t, demoted)
	assert.Equal(t, int64(0), p)
}

func TestRecordSlowStream_FastSampleDoesNotCancelDemotion(t *testing.T) {
	setupTest(t, true)
	// 连续 3 次慢速触发降级
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.True(t, RecordSlowStream(1, "gpt-4o", 1.0))
	demoted, p := GetDemotedPriority(1, "gpt-4o", 5)
	assert.True(t, demoted)
	assert.Equal(t, int64(0), p)
	// 一次快请求：只重置窗口计数，不得取消进行中的降级
	assert.False(t, RecordSlowStream(1, "gpt-4o", 10.0))
	demoted, p = GetDemotedPriority(1, "gpt-4o", 5)
	assert.True(t, demoted, "fast sample must not cancel an active demotion")
	assert.Equal(t, int64(0), p)
	// 快请求后窗口计数已重置：需重新累计 Threshold 次才触发新一轮降级
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
}

func TestRecordSlowStream_ExcludedChannel_ReturnsFalse(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.Threshold = 1
	cfg.ExcludeChannelIDs = []int{7}
	// 排除渠道：即使 TPS 远低于阈值也不计数、不降级
	assert.False(t, RecordSlowStream(7, "gpt-4o", 0.1))
	assert.False(t, RecordSlowStream(7, "gpt-4o", 0.1))
	assert.Nil(t, loadState(t, 7, "gpt-4o"))
	demoted, p := GetDemotedPriority(7, "gpt-4o", 5)
	assert.False(t, demoted)
	assert.Equal(t, int64(5), p)
	// 非排除渠道不受影响
	assert.True(t, RecordSlowStream(8, "gpt-4o", 0.1))
}

func TestGetDemotedPriority_ExcludedChannel_ReturnsOriginal(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.Threshold = 1
	// 先让渠道 9 触发降级
	assert.True(t, RecordSlowStream(9, "gpt-4o", 0.1))
	demoted, p := GetDemotedPriority(9, "gpt-4o", 5)
	assert.True(t, demoted)
	assert.Equal(t, int64(0), p)
	// 将渠道 9 加入排除列表后，历史降级记录不再生效
	cfg.ExcludeChannelIDs = []int{9}
	demoted, p = GetDemotedPriority(9, "gpt-4o", 5)
	assert.False(t, demoted)
	assert.Equal(t, int64(5), p)
}

func TestGetDemotedPriority_NotDemoted_ReturnsOriginal(t *testing.T) {
	setupTest(t, true)
	demoted, p := GetDemotedPriority(1, "gpt-4o", 5)
	assert.False(t, demoted)
	assert.Equal(t, int64(5), p)
}

func TestGetDemotedPriority_Demoted_ReturnsMinPriority(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.DemotedPriority = 0
	// 触发降级
	RecordSlowStream(1, "gpt-4o", 1.0)
	RecordSlowStream(1, "gpt-4o", 1.0)
	assert.True(t, RecordSlowStream(1, "gpt-4o", 1.0))
	demoted, p := GetDemotedPriority(1, "gpt-4o", 5)
	assert.True(t, demoted)
	assert.Equal(t, int64(0), p)
	// min 语义：原 priority 低于 DemotedPriority 时返回原值
	cfg.DemotedPriority = 3
	demoted, p = GetDemotedPriority(1, "gpt-4o", 5)
	assert.True(t, demoted)
	assert.Equal(t, int64(3), p)
	demoted, p = GetDemotedPriority(1, "gpt-4o", 1)
	assert.True(t, demoted)
	assert.Equal(t, int64(1), p)
}

func TestGetDemotedPriority_Expired_ReturnsOriginal(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.DemoteDurationSec = 1
	RecordSlowStream(1, "gpt-4o", 1.0)
	RecordSlowStream(1, "gpt-4o", 1.0)
	assert.True(t, RecordSlowStream(1, "gpt-4o", 1.0))
	demoted, _ := GetDemotedPriority(1, "gpt-4o", 5)
	assert.True(t, demoted)
	// 等待降级到期 → 自动恢复
	time.Sleep(1100 * time.Millisecond)
	demoted, p := GetDemotedPriority(1, "gpt-4o", 5)
	assert.False(t, demoted)
	assert.Equal(t, int64(5), p)
}

func TestCleanupExpiredDemotions(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.DemoteDurationSec = 1
	// 触发一个降级（channel 1，3 次慢速）+ 一个未降级计数（channel 2，2 次慢速）
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.True(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(2, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(2, "gpt-4o", 1.0))
	time.Sleep(1100 * time.Millisecond)
	CleanupExpiredDemotions()
	// 降级到期的条目被清除；未降级条目的计数保留（固定数量窗口不会自行过期）
	require.Nil(t, loadState(t, 1, "gpt-4o"))
	state := loadState(t, 2, "gpt-4o")
	require.NotNil(t, state)
	assert.Equal(t, 2, state.slowCount)
}

func TestRecordSlowTtft_TriggersDemotionAfterThreshold(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.TtftThreshold = 1
	// 首字 6s（超过 MaxTtftMs=5000）即触发降级
	assert.True(t, RecordSlowTtft(1, "gpt-4o", 6000))
	demoted, p := GetDemotedPriority(1, "gpt-4o", 5)
	assert.True(t, demoted)
	assert.Equal(t, int64(0), p)
	// 首字未超阈值：不触发，且重置计数
	assert.False(t, RecordSlowTtft(1, "gpt-4o", 1000))
	demoted, p = GetDemotedPriority(1, "gpt-4o", 5)
	assert.True(t, demoted, "fast ttft must not cancel an active demotion")
	assert.Equal(t, int64(0), p)
}

func TestRecordSlowTtft_IndependentFromGenerationRate(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.Threshold = 3
	cfg.TtftThreshold = 1
	// 仅 TTFT 触发降级（生成速率正常，不参与 TTFT 降级判定）
	assert.True(t, RecordSlowTtft(2, "gpt-4o", 6000))
	demoted, p := GetDemotedPriority(2, "gpt-4o", 5)
	assert.True(t, demoted)
	assert.Equal(t, int64(0), p)
	// 生成速率慢未达阈值，不影响 TTFT 降级状态
	assert.False(t, RecordSlowStream(2, "gpt-4o", 1.0))
	demoted, _ = GetDemotedPriority(2, "gpt-4o", 5)
	assert.True(t, demoted)
}

func TestRecordSlowTtft_ExcludedChannel_ReturnsFalse(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.TtftThreshold = 1
	cfg.ExcludeChannelIDs = []int{7}
	assert.False(t, RecordSlowTtft(7, "gpt-4o", 6000))
	demoted, p := GetDemotedPriority(7, "gpt-4o", 5)
	assert.False(t, demoted)
	assert.Equal(t, int64(5), p)
	// 非排除渠道不受影响
	assert.True(t, RecordSlowTtft(8, "gpt-4o", 6000))
}

func TestRecordSlowTtft_Disabled_ReturnsFalse(t *testing.T) {
	setupTest(t, false)
	assert.False(t, RecordSlowTtft(1, "gpt-4o", 6000))
	assert.False(t, RecordSlowTtft(1, "gpt-4o", 10000))
}

func TestListDemoted_MemoryMode(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.Threshold = 1
	// 渠道 1（gpt-4o）、渠道 2（gpt-4o + claude-3-5）触发降级
	assert.True(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.True(t, RecordSlowStream(2, "gpt-4o", 1.0))
	assert.True(t, RecordSlowStream(2, "claude-3-5", 1.0))
	demoted := ListDemoted()
	require.Len(t, demoted[1], 1)
	require.Len(t, demoted[2], 2)
	assert.Equal(t, "gpt-4o", demoted[1][0].Model)
	assert.Greater(t, demoted[1][0].RemainingSeconds, int64(0))
	// 非降级渠道不出现
	require.NotContains(t, demoted, 3)
	// 默认 map 无降级时为空 map（controller 兜底也可用）
	delete(demoted, 1)
	delete(demoted, 2)
	require.Empty(t, demoted)
}

func TestListDemoted_MergesBothSourcesPerModel(t *testing.T) {
	// 同一 (channelId, model) 两源同时降级必须合并为一条记录，
	// Sources 同时含 tps 与 ttft（前端悬停展示降级原因依赖此字段）。
	cfg := setupTest(t, true)
	cfg.Threshold = 1
	cfg.TtftThreshold = 1
	assert.True(t, RecordSlowStream(7, "gpt-4o", 1.0))
	assert.True(t, RecordSlowTtft(7, "gpt-4o", 9000))

	demoted := ListDemoted()
	require.Len(t, demoted[7], 1)
	info := demoted[7][0]
	assert.Equal(t, "gpt-4o", info.Model)
	assert.ElementsMatch(t, []string{DemotionSourceTps, DemotionSourceTtft}, info.Sources)
	assert.Greater(t, info.RemainingSeconds, int64(0))

	// 单源降级只报该来源
	assert.True(t, RecordSlowStream(8, "gpt-4o", 1.0))
	demoted = ListDemoted()
	require.Len(t, demoted[8], 1)
	assert.Equal(t, []string{DemotionSourceTps}, demoted[8][0].Sources)
}

func TestListDemoted_AllDisabled_ReturnsEmpty(t *testing.T) {
	setupTest(t, false)
	assert.Nil(t, ListDemoted())
}
