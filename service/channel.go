package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, common.LocalLogPreview(reason)))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

// DisableDecision carries the result of channel disable evaluation, including
// whether the sliding window threshold was reached and a human-readable reason.
type DisableDecision struct {
	ShouldDisable bool   // true if the channel should be disabled
	Reason        string // human-readable disable reason
}

// ShouldDisableChannelWithDecision evaluates whether a channel should be
// disabled based on the error, applying sliding-window counting. The
// channelID is used as the error identity key so that different channels are
// counted independently.
func ShouldDisableChannelWithDecision(channelID int, err *types.NewAPIError) DisableDecision {
	if !common.AutomaticDisableChannelEnabled {
		return DisableDecision{}
	}
	if err == nil {
		return DisableDecision{}
	}

	isConfigured := false
	reason := ""

	if types.IsChannelError(err) {
		isConfigured = true
		reason = err.ErrorWithStatusCode()
	} else if types.IsSkipRetryError(err) {
		return DisableDecision{}
	} else if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		isConfigured = true
		reason = err.ErrorWithStatusCode()
	} else {
		lowerMessage := strings.ToLower(err.Error())
		matched, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
		if matched {
			isConfigured = true
			reason = err.ErrorWithStatusCode()
		}
	}

	if isConfigured {
		if triggered, detail := CheckAndRecordDisable(channelID, err.StatusCode, true); triggered {
			return DisableDecision{ShouldDisable: true, Reason: fmt.Sprintf("%s; %s", reason, detail)}
		}
		return DisableDecision{}
	}

	// Unconfigured errors: only count valid HTTP status codes (1xx-5xx).
	if err.StatusCode < 100 || err.StatusCode > 599 {
		return DisableDecision{}
	}
	if triggered, detail := CheckAndRecordDisable(channelID, err.StatusCode, false); triggered {
		return DisableDecision{
			ShouldDisable: true,
			Reason:        fmt.Sprintf("channel disabled: status_code=%d (%s)", err.StatusCode, detail),
		}
	}
	return DisableDecision{}
}

// ShouldDisableChannel is a backwards-compatible wrapper that returns only
// the boolean decision. New callers should use ShouldDisableChannelWithDecision.
func ShouldDisableChannel(channelID int, err *types.NewAPIError) bool {
	return ShouldDisableChannelWithDecision(channelID, err).ShouldDisable
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
