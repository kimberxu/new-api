/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	channelslowstream "github.com/QuantumNous/new-api/pkg/channel_slowstream"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
)

func setupTtftSampleTest(t *testing.T) {
	t.Helper()
	origRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = origRedis })

	cfg := config.GlobalConfig.Get("channel_slow_stream_setting").(*operation_setting.ChannelSlowStreamSetting)
	orig := *cfg
	*cfg = operation_setting.ChannelSlowStreamSetting{
		Enabled:           false, // 只测 TTFT 源，避免生成速率采样干扰
		TtftEnabled:       true,
		MaxTtftMs:         1000,
		TtftThreshold:     1,
		TtftSampleSize:    5,
		TtftWindowSeconds: 300,
		DemoteDurationSec: 600,
		MinInputTokens:    100,
	}
	t.Cleanup(func() { *cfg = orig })
}

func makeStreamRelayInfo(channelId int, frtMs int64) *relaycommon.RelayInfo {
	now := time.Now()
	return &relaycommon.RelayInfo{
		IsStream:          true,
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: channelId},
		StartTime:         now.Add(-2 * time.Second),
		FirstResponseTime: now.Add(-2*time.Second + time.Duration(frtMs)*time.Millisecond),
		OriginModelName:   "gpt-4o",
	}
}

// TestRecordFromRelayInfoTtftMinInputTokens verifies the MinInputTokens gate:
// requests below the input threshold are not sampled for TTFT demotion, while
// requests at or above it are (given a slow enough first-token latency).
func TestRecordFromRelayInfoTtftMinInputTokens(t *testing.T) {
	setupTtftSampleTest(t)

	// 输入 token 低于门槛：frt 超阈值也不采样、不降级
	RecordFromRelayInfo(makeStreamRelayInfo(5, 5000), 200, 10)
	demoted, p := channelslowstream.GetDemotedPriority(5, "gpt-4o", 5)
	assert.False(t, demoted)
	assert.Equal(t, int64(5), p)

	// 输入 token 达到门槛：frt 超阈值 → 采样并降级
	RecordFromRelayInfo(makeStreamRelayInfo(5, 5000), 200, 100)
	demoted, p = channelslowstream.GetDemotedPriority(5, "gpt-4o", 5)
	assert.True(t, demoted)
	assert.Equal(t, int64(0), p)
}