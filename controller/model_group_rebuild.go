// model_group_rebuild.go — "auto rebuild routing" action for model groups.
// [personal] Rebuilds model-group routing across all enabled channels,
// respecting each channel's "auto sync upstream models" switch.
package controller

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// RebuildModelGroups rebuilds model-group routing across all enabled channels.
// For each channel it follows the auto-sync switch:
//   - auto_sync ON  → apply pending upstream changes (adds merged into
//     channel.Models, detected removals dropped) then sync the channel's
//     groups (applyChannelUpstreamModelUpdates already calls
//     SyncModelGroupForChannel internally).
//   - auto_sync OFF → use the channel's configured models as-is: sync its
//     groups without triggering detection and without touching channel.Models
//     ("沿用此前渠道设置中模型配置，没有变化").
//
// Manual edits inside auto groups are preserved (OnConflict DoNothing in
// AddModelGroupItems); the source field is never flipped. Reference parents
// pick up child changes automatically at read time via
// ListModelGroupItemsExpanded, so no physical parent rewrite is needed.
func RebuildModelGroups(c *gin.Context) {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var onCount, offCount int
	for _, ch := range channels {
		settings := ch.GetOtherSettings()
		if !settings.UpstreamModelUpdateAutoSyncEnabled {
			// Switch off: keep using configured models, no detection,
			// no change to channel.Models.
			if serr := model.SyncModelGroupForChannel(ch); serr != nil {
				common.SysLog(fmt.Sprintf("rebuild: failed to sync channel #%d: %v", ch.Id, serr))
			}
			offCount++
			continue
		}

		// Switch on: apply pending upstream add/remove, then sync groups.
		_, _, _, _, _, aerr := applyChannelUpstreamModelUpdates(
			ch,
			settings.UpstreamModelUpdateLastDetectedModels,
			nil,
			settings.UpstreamModelUpdateLastRemovedModels,
		)
		if aerr != nil {
			common.SysLog(fmt.Sprintf("rebuild: failed to apply upstream for channel #%d: %v", ch.Id, aerr))
			continue
		}
		// applyChannelUpstreamModelUpdates only syncs groups when models
		// actually changed; reconcile unconditionally so a rebuild always
		// repairs group membership even with empty pending lists.
		if serr := model.SyncModelGroupForChannel(ch); serr != nil {
			common.SysLog(fmt.Sprintf("rebuild: failed to sync channel #%d: %v", ch.Id, serr))
		}
		onCount++
	}
	// Drop auto groups whose members all disappeared (model no longer served
	// by any channel); dangling references go with them via DeleteModelGroup.
	removed, rerr := model.DeleteEmptyAutoModelGroups()
	if rerr != nil {
		common.SysLog(fmt.Sprintf("rebuild: failed to clean up empty auto groups: %v", rerr))
	}
	if len(removed) > 0 {
		common.SysLog(fmt.Sprintf("rebuild: removed %d empty auto group(s): %v", len(removed), removed))
	}

	model.InitChannelCache()

	common.ApiSuccess(c, gin.H{
		"rebuilt":        true,
		"on_channels":    onCount,
		"off_channels":   offCount,
		"removed_groups": removed,
	})
}
