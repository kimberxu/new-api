package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// ChannelSlowStreamSetting 渠道流式速率降级全局配置。
// 检测到某渠道在窗口内多个成功流式请求 TPS 持续低于 MinTps 时，
// 临时将该渠道在对应模型上的优先级拍平到 DemotedPriority，到期自动恢复。
type ChannelSlowStreamSetting struct {
	Enabled           bool    `json:"enabled"`             // 全局开关，默认 true
	MinTps            float64 `json:"min_tps"`             // TPS 下限 tokens/s，默认 8.0
	WindowSeconds     int64   `json:"window_seconds"`      // 滑动窗口秒数，默认 300
	Threshold         int     `json:"threshold"`           // 窗口内连续慢速次数触发降级，默认 1
	MinOutputTokens   int64   `json:"min_output_tokens"`   // 最小输出 token 数门槛，避免短请求噪声，默认 50
	DemoteDurationSec int64   `json:"demote_duration_sec"` // 降级持续时间秒，默认 600
	DemotedPriority   int64   `json:"demoted_priority"`    // 降级后优先级（拍平到此值），默认 0
	ExcludeChannelIDs []int   `json:"exclude_channel_ids"` // 排除渠道编号列表，不参与慢流式降级
}

var channelSlowStreamSetting = ChannelSlowStreamSetting{
	Enabled:           true,
	MinTps:            8.0,
	WindowSeconds:     300,
	Threshold:         1,
	MinOutputTokens:   50,
	DemoteDurationSec: 600,
	DemotedPriority:   0,
}

func init() {
	config.GlobalConfig.Register("channel_slow_stream_setting", &channelSlowStreamSetting)
}

func GetChannelSlowStreamSetting() ChannelSlowStreamSetting {
	return channelSlowStreamSetting
}
