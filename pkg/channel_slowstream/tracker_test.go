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
		Threshold:         3,
		MinOutputTokens:   50,
		DemoteDurationSec: 600,
		DemotedPriority:   0,
	}
	t.Cleanup(func() {
		*cfg = orig
		slowStreamMap = sync.Map{}
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

func TestRecordSlowStream_ResetOnFastSample(t *testing.T) {
	setupTest(t, true)
	// 两次慢速
	RecordSlowStream(1, "gpt-4o", 1.0)
	RecordSlowStream(1, "gpt-4o", 1.0)
	// 一次不慢 → 重置窗口计数
	assert.False(t, RecordSlowStream(1, "gpt-4o", 10.0))
	// 重置后需重新累计 Threshold 次才触发
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.True(t, RecordSlowStream(1, "gpt-4o", 1.0))
	// 不同 (channelId, model) 独立计数
	assert.False(t, RecordSlowStream(1, "claude-3-5", 1.0))
	assert.False(t, RecordSlowStream(2, "gpt-4o", 1.0))
}

func TestRecordSlowStream_WindowExpiryResetsCount(t *testing.T) {
	cfg := setupTest(t, true)
	cfg.WindowSeconds = 1
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	// 窗口过期 → 计数重置，重新累计
	// 注意：unix 秒级时间戳，sleep 2.1s 保证 now-lastSlowAt >= 2 > WindowSeconds
	time.Sleep(2100 * time.Millisecond)
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.False(t, RecordSlowStream(1, "gpt-4o", 1.0))
	assert.True(t, RecordSlowStream(1, "gpt-4o", 1.0))
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
	// 降级到期的条目被清除；未降级条目的计数保留（窗口未过期）
	require.Nil(t, loadState(t, 1, "gpt-4o"))
	state := loadState(t, 2, "gpt-4o")
	require.NotNil(t, state)
	assert.Equal(t, 2, state.count)
}
