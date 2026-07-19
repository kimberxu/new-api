package service

import (
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTextOtherInfoAddsRequestDebugToAdminInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := &gin.Context{}
	start := time.Unix(100, 0)
	info := &relaycommon.RelayInfo{
		StartTime:         start,
		FirstResponseTime: start.Add(time.Second),
		ChannelMeta:       &relaycommon.ChannelMeta{},
		RequestDebugSnapshot: &relaycommon.RequestDebugSnapshot{
			Mode: "always",
			Downstream: &relaycommon.RequestDebugBody{
				Size: 4,
				Body: "{}",
			},
		},
	}

	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 1)

	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Same(t, info.RequestDebugSnapshot, adminInfo["request_debug"])
}

func TestGenerateTextOtherInfoSkipsErrorOnlyRequestDebugOnSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := &gin.Context{}
	start := time.Unix(100, 0)
	info := &relaycommon.RelayInfo{
		StartTime:         start,
		FirstResponseTime: start.Add(time.Second),
		ChannelMeta:       &relaycommon.ChannelMeta{},
		RequestDebugSnapshot: &relaycommon.RequestDebugSnapshot{
			Mode: "error_only",
			Downstream: &relaycommon.RequestDebugBody{
				Size: 2,
				Body: "{}",
			},
		},
	}

	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 1)

	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	_, exists := adminInfo["request_debug"]
	assert.False(t, exists)
}

func TestAppendRequestDebugAdminInfoAddsErrorOnlySnapshotOnFailure(t *testing.T) {
	snapshot := &relaycommon.RequestDebugSnapshot{Mode: "error_only"}
	info := &relaycommon.RelayInfo{RequestDebugSnapshot: snapshot}
	adminInfo := map[string]interface{}{}

	AppendRequestDebugAdminInfo(info, adminInfo, false)

	assert.Same(t, snapshot, adminInfo["request_debug"])
}
