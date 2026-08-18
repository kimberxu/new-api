package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// ChannelSlowStreamSetting 渠道流式速率降级全局配置。
// 固定数量窗口（ring buffer）：每个 (channelId, model) 保留最近 SampleSize 次
// 采样结果（快慢都记），其中慢事件次数达到 Threshold 即触发降级；
// 快事件只挤掉最旧样本，不洗白历史慢记录。
type ChannelSlowStreamSetting struct {
	Enabled           bool    `json:"enabled"`             // 生成速率降级总开关，默认 true
	MinTps            float64 `json:"min_tps"`             // 生成 TPS 下限 tokens/s，默认 8.0
	WindowSeconds     int64   `json:"window_seconds"`      // [废弃] 时间窗口秒数，2026-08-17 起不再生效，保留兼容旧配置
	SampleSize        int     `json:"sample_size"`         // 保留最近采样次数（ring buffer 容量），默认 5
	Threshold         int     `json:"threshold"`           // 窗口内慢事件次数触发降级，默认 3
	MinOutputTokens   int64   `json:"min_output_tokens"`   // 生成速率最小输出 token 数门槛，避免短请求噪声，默认 50
	MinInputTokens    int64   `json:"min_input_tokens"`    // TTFT 采样最小输入 token 数门槛，过滤短请求噪声，默认 0（不启用）
	DemoteDurationSec int64   `json:"demote_duration_sec"` // 降级持续时间秒，默认 600
	DemotedPriority   int64   `json:"demoted_priority"`    // 降级后优先级（拍平到此值），默认 0
	ExcludeChannelIDs []int   `json:"exclude_channel_ids"` // 排除渠道编号列表，不参与慢流式/首字延迟降级

	TtftEnabled       bool  `json:"ttft_enabled"`        // 首字延迟（TTFT）降级开关，默认 true
	MaxTtftMs         int64 `json:"max_ttft_ms"`         // 首字延迟上限毫秒，超过计为慢事件，默认 5000
	TtftWindowSeconds int64 `json:"ttft_window_seconds"` // [废弃] 首字延迟时间窗口秒数，2026-08-17 起不再生效，保留兼容旧配置
	TtftSampleSize    int   `json:"ttft_sample_size"`    // 保留最近采样次数（ring buffer 容量），默认 5
	TtftThreshold     int   `json:"ttft_threshold"`      // 窗口内慢首字次数触发降级，默认 3
}

var channelSlowStreamSetting = ChannelSlowStreamSetting{
	Enabled:           true,
	MinTps:            8.0,
	WindowSeconds:     300,
	SampleSize:        5,
	Threshold:         3,
	MinOutputTokens:   50,
	DemoteDurationSec: 600,
	DemotedPriority:   0,

	TtftEnabled:       true,
	MaxTtftMs:         5000,
	TtftWindowSeconds: 300,
	TtftSampleSize:    5,
	TtftThreshold:     3,
}

func init() {
	config.GlobalConfig.Register("channel_slow_stream_setting", &channelSlowStreamSetting)
}

func GetChannelSlowStreamSetting() ChannelSlowStreamSetting {
	return channelSlowStreamSetting
}
