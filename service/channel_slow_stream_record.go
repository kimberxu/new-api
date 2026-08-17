package service

import (
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	channelslowstream "github.com/QuantumNous/new-api/pkg/channel_slowstream"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// RecordFromRelayInfo 从请求结束的 RelayInfo 采样一次流速率。
// 只统计成功流式请求；失败流式、非流式、无 channel、无有效 generation 时长、
// 输出 token 低于 MinOutputTokens 的请求一律跳过。
func RecordFromRelayInfo(info *relaycommon.RelayInfo, outputTokens int64) {
	if info == nil || !info.IsStream || !info.StreamSucceeded() {
		return
	}
	channelId := info.GetChannelID()
	if channelId <= 0 {
		return
	}
	setting := operation_setting.GetChannelSlowStreamSetting()
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
	channelslowstream.RecordSlowStream(channelId, info.OriginModelName, tps)
}
