package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	channelslowstream "github.com/QuantumNous/new-api/pkg/channel_slowstream"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRandomSatisfiedChannelSlowStreamDemotion(t *testing.T) {
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	// 打开慢速降级开关
	cfg := config.GlobalConfig.Get("channel_slow_stream_setting").(*operation_setting.ChannelSlowStreamSetting)
	origCfg := *cfg
	*cfg = operation_setting.ChannelSlowStreamSetting{
		Enabled:           true,
		MinTps:            5.0,
		WindowSeconds:     300,
		Threshold:         3,
		MinOutputTokens:   50,
		DemoteDurationSec: 600,
		DemotedPriority:   0,
	}
	t.Cleanup(func() { *cfg = origCfg })

	insertChannelSelectionTestData(t, []struct {
		id       int
		priority int64
		weight   uint
	}{
		{id: 401, priority: 5, weight: 1},
		{id: 402, priority: 5, weight: 1},
	})
	InitChannelCache()

	// 未降级：两个渠道同层，均可能被选中
	ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, "", nil)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.Contains(t, []int{401, 402}, ch.Id)

	// 触发渠道 401 降级（连续 3 次慢速）
	assert.False(t, channelslowstream.RecordSlowStream(401, "test-model", 1.0))
	assert.False(t, channelslowstream.RecordSlowStream(401, "test-model", 1.0))
	assert.True(t, channelslowstream.RecordSlowStream(401, "test-model", 1.0))

	// 降级后：多次调用应始终返回 402（独占最高优先级层 5），401 跌到 0 层
	for range 10 {
		ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, "", nil)
		require.NoError(t, err)
		require.NotNil(t, ch)
		assert.Equal(t, 402, ch.Id, "demoted channel 401 must not be selected while 402 is available")
	}

	// 排除 402 后：401 成为唯一最高层（0 层），级联选中，不会永久饿死
	ch, err = GetRandomSatisfiedChannel("default", "test-model", 0, "", []int{402})
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, 401, ch.Id, "demoted channel must be selectable after higher tier exhausted")
}
