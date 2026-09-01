package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

// [personal] modelBanRecoveryDecision captures the outcome of one expired
// model-ban recovery probe.
type modelBanRecoveryDecision struct {
	// Recovered: the probe succeeded and the ban record was cleared.
	Recovered bool
	// Extended: the probe failed and the ban deadline was renewed.
	Extended bool
	// Skipped: the record was not eligible (channel gone / manually disabled).
	Skipped bool
}

// decideModelBanRecovery is a pure function (unit-testable): given an expired
// auto ban record and the probe outcome, what should happen?
func decideModelBanRecovery(record *model.ChannelDisabledModel, probeOK bool) modelBanRecoveryDecision {
	if record == nil || record.Source != "auto" || record.BannedUntil <= 0 {
		return modelBanRecoveryDecision{Skipped: true}
	}
	if probeOK {
		return modelBanRecoveryDecision{Recovered: true}
	}
	return modelBanRecoveryDecision{Extended: true}
}

// recoverExpiredModelBans probes model-level auto bans whose deadline has
// passed and recovers or extends them. It is a targeted, per-model pass:
// exactly one request per expired ban, never a full channel model sweep.
// Channel-level recovery stays on the existing passive-recovery path.
// Returns the number of recovered models.
func recoverExpiredModelBans(ctx context.Context, testUserID int) (int, error) {
	all, err := model.GetAllChannelDisabledModels()
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	var expired []model.ChannelDisabledModel
	for _, record := range all {
		if record.Source == "auto" && record.BannedUntil > 0 && record.BannedUntil <= now {
			expired = append(expired, record)
		}
	}
	if len(expired) == 0 {
		return 0, nil
	}

	recovered := 0
	for _, record := range expired {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		logger.LogDebug(ctx, "recoverExpiredModelBans: probing channel #%d model %s (banned_until=%d)", record.ChannelId, record.Model, record.BannedUntil)

		channel, err := model.GetChannelById(record.ChannelId, true)
		if err != nil || channel == nil {
			// Channel gone: drop the stale record so it stops resurfacing.
			_ = model.EnableChannelModelDisabled(record.ChannelId, record.Model, "auto")
			continue
		}
		// Never probe channels the admin has manually disabled.
		if channel.Status == common.ChannelStatusManuallyDisabled {
			continue
		}

		result := testChannel(ctx, channel, testUserID, record.Model, "", shouldUseStreamForAutomaticChannelTest(channel))
		decision := decideModelBanRecovery(&record, result.localErr == nil)
		switch {
		case decision.Recovered:
			if err := service.EnableChannelModel(record.ChannelId, record.Model, "auto"); err != nil {
				logger.LogError(ctx, fmt.Sprintf("recoverExpiredModelBans: enable failed channel #%d model %s: %v", record.ChannelId, record.Model, err))
				continue
			}
			recovered++
			logger.LogDebug(ctx, "recoverExpiredModelBans: recovered channel #%d model %s", record.ChannelId, record.Model)
		case decision.Extended:
			if err := service.ExtendChannelModelBan(record.ChannelId, record.Model, result.localErr.Error()); err != nil {
				logger.LogError(ctx, fmt.Sprintf("recoverExpiredModelBans: extend failed channel #%d model %s: %v", record.ChannelId, record.Model, err))
				continue
			}
			logger.LogDebug(ctx, "recoverExpiredModelBans: extended ban channel #%d model %s", record.ChannelId, record.Model)
		}

		if common.RequestInterval > 0 {
			if ctx == nil {
				time.Sleep(common.RequestInterval)
			} else {
				select {
				case <-ctx.Done():
					return recovered, nil
				case <-time.After(common.RequestInterval):
				}
			}
		}
	}
	return recovered, nil
}