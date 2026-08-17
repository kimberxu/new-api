package service

import (
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	channelslowstream "github.com/QuantumNous/new-api/pkg/channel_slowstream"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// RecordFromRelayInfo 从请求结束的 RelayInfo 采样生成速率与首字延迟。
// 只统计成功流式请求；失败流式、非流式、无 channel 一律跳过。
func RecordFromRelayInfo(info *relaycommon.RelayInfo, outputTokens int64, inputTokens int64) {
	if info == nil || !info.IsStream || !info.StreamSucceeded() {
		return
	}
	channelId := info.GetChannelID()
	if channelId <= 0 {
		return
	}
	setting := operation_setting.GetChannelSlowStreamSetting()
	if !setting.Enabled && !setting.TtftEnabled {
		return
	}
	model := info.OriginModelName

	// 首字延迟（TTFT）采样：frt = FirstResponseTime - StartTime（毫秒）。
	// MinInputTokens 过滤短请求噪声：短输入的 frt 基本不受 prefill 影响，
	// 命中阈值多为抖动；长输入的 frt 才是用户关心的「首字等太久」体验。
	if setting.TtftEnabled && inputTokens >= setting.MinInputTokens {
		frtMs := info.FirstResponseTime.Sub(info.StartTime).Milliseconds()
		if frtMs > 0 {
			channelslowstream.RecordSlowTtft(channelId, model, frtMs)
		}
	}

	if !setting.Enabled {
		return
	}
	if outputTokens < setting.MinOutputTokens {
		return
	}
	genMs := time.Now().Sub(info.FirstResponseTime).Milliseconds()
	if genMs <= 0 {
		return
	}
	tps := float64(outputTokens) / float64(genMs) * 1000.0
	channelslowstream.RecordSlowStream(channelId, model, tps)
}
