package service

import (
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// modelBanDurations is the escalating auto-ban duration ladder.
// Stage index 0..6 → 30min, 1h, 2h, 4h, 8h, 16h, 32h; stage 7 = permanent.
var modelBanDurations = []time.Duration{
	30 * time.Minute,
	1 * time.Hour,
	2 * time.Hour,
	4 * time.Hour,
	8 * time.Hour,
	16 * time.Hour,
	32 * time.Hour,
}

// initialBanStageForStatusCode returns the first ban stage: 401 starts at
// stage 5 (16h), all other errors at stage 0 (30min).
func initialBanStageForStatusCode(statusCode int) int {
	if statusCode == http.StatusUnauthorized {
		return 5
	}
	return 0
}

// DisableChannelModel disables a single model on a channel (source=auto) and
// rebuilds the channel cache so routing excludes the pair immediately. The
// initial ban stage is determined by statusCode: 401 errors start at stage 5
// (16h), all other errors at stage 0 (30min).
func DisableChannelModel(channelID int, modelName string, reason string, statusCode int, lastError string) error {
	common.SysLog(fmt.Sprintf("通道 #%d 模型 %s 请求失败，准备禁用该模型，原因：%s", channelID, modelName, common.LocalLogPreview(reason)))

	now := time.Now()
	stage := initialBanStageForStatusCode(statusCode)
	bannedUntil := now.Add(modelBanDurations[stage]).Unix()
	if err := model.AddChannelDisabledModels(channelID, []string{modelName}, "auto", reason); err != nil {
		common.SysLog(fmt.Sprintf("failed to add channel disabled model: channel_id=%d, model=%s, error=%v", channelID, modelName, err))
		return err
	}
	if err := model.SetChannelDisabledModelBanStage(channelID, modelName, stage, bannedUntil, lastError); err != nil {
		common.SysLog(fmt.Sprintf("failed to set ban stage: channel_id=%d, model=%s, error=%v", channelID, modelName, err))
		return err
	}
	model.InitChannelCache()

	channel, err := model.GetChannelById(channelID, false)
	if err == nil && channel != nil {
		subject := fmt.Sprintf("通道「%s」（#%d）模型 %s 已被禁用", channel.Name, channelID, modelName)
		content := fmt.Sprintf("通道「%s」（#%d）模型 %s 已被禁用，原因：%s", channel.Name, channelID, modelName, reason)
		NotifyRootUser(formatNotifyType(channelID, common.ChannelStatusAutoDisabled), subject, content)
	}
	return nil
}

// EnableChannelModel re-enables a single model on a channel. An empty source
// clears any source; "auto" only clears auto-sourced disables.
func EnableChannelModel(channelID int, modelName string, source string) error {
	if err := model.EnableChannelModelDisabled(channelID, modelName, source); err != nil {
		common.SysLog(fmt.Sprintf("failed to enable channel disabled model: channel_id=%d, model=%s, error=%v", channelID, modelName, err))
		return err
	}
	model.InitChannelCache()
	return nil
}

// ExtendChannelModelBan escalates the ban of an auto model-level disable
// record (used when the recovery probe still fails). Stage advances one step
// per call; stage 7 (newStage >= len(modelBanDurations)) means permanent
// (BannedUntil=0), and the recovery probe will not re-test. lastError is
// the upstream error observed by the failing probe, persisted so the UI can
// explain the extension.
func ExtendChannelModelBan(channelID int, modelName string, lastError string) error {
	record, err := model.GetChannelDisabledModel(channelID, modelName)
	if err != nil || record == nil {
		return nil // record gone (manual clear) — nothing to extend
	}
	newStage := record.BanStage + 1
	bannedUntil := int64(0) // permanent
	if newStage < len(modelBanDurations) {
		bannedUntil = time.Now().Add(modelBanDurations[newStage]).Unix()
	}
	// newStage == len(modelBanDurations) (7) ⇒ bannedUntil stays 0 = permanent
	if err := model.SetChannelDisabledModelBanStage(channelID, modelName, newStage, bannedUntil, lastError); err != nil {
		common.SysLog(fmt.Sprintf("failed to extend channel disabled model ban: channel_id=%d, model=%s, error=%v", channelID, modelName, err))
		return err
	}
	return nil
}
