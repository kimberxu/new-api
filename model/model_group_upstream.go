package model

import (
	"github.com/QuantumNous/new-api/common"
)

// [personal] Model-group members carry the real upstream model on their
// channel. The group name is only the downstream-facing routable model name
// and may not exist upstream at all (e.g. a manual "ox" group), so before
// relaying, the member's upstream model must replace the requested name.
//
// We reuse the existing channel model_mapping pipeline instead of touching
// relay: ApplyModelGroupMemberMapping merges {routableModel: upstreamModel}
// into the channel's mapping JSON inside SetupContextForSelectedChannel, and
// ModelMappedHelper does the rewrite as usual. An explicit channel-level
// mapping entry for the same name always wins.

// ResolveModelGroupUpstreamModel returns the real upstream model that the
// enabled member (routableModel, channelId) records, or "" when no rewrite is
// needed: no such member, or the member model equals the group name (auto
// groups). Uses the routing cache when the memory cache is on, else queries
// the tables directly (DB-fallback selection mode).
func ResolveModelGroupUpstreamModel(routableModel string, channelId int) string {
	if common.MemoryCacheEnabled {
		channelSyncLock.RLock()
		defer channelSyncLock.RUnlock()
		for _, perChannel := range modelGroupItemOverrides[routableModel] {
			if o, ok := perChannel[channelId]; ok && o.model != "" && o.model != routableModel {
				return o.model
			}
		}
		return ""
	}
	var m string
	err := DB.Table("model_group_items").
		Select("model_group_items.model").
		Joins("JOIN model_groups ON model_group_items.group_id = model_groups.id").
		Where("model_groups.name = ? AND model_group_items.channel_id = ? AND model_group_items.enabled = ?",
			routableModel, channelId, true).
		Take(&m).Error
	if err != nil || m == "" || m == routableModel {
		return ""
	}
	return m
}

// ApplyModelGroupMemberMapping merges the member's upstream model into the
// channel model_mapping JSON. Values may be weighted arrays; they are
// preserved via interface{} round-tripping. Unparseable existing JSON is
// returned untouched (ModelMappedHelper reports it later).
func ApplyModelGroupMemberMapping(mappingJSON string, routableModel string, channelId int) string {
	upstream := ResolveModelGroupUpstreamModel(routableModel, channelId)
	if upstream == "" {
		return mappingJSON
	}
	modelMap := make(map[string]interface{}, 2)
	if mappingJSON != "" && mappingJSON != "{}" {
		if err := common.UnmarshalJsonStr(mappingJSON, &modelMap); err != nil {
			return mappingJSON
		}
	}
	if _, ok := modelMap[routableModel]; ok {
		return mappingJSON // explicit channel mapping wins
	}
	modelMap[routableModel] = upstream
	data, err := common.Marshal(modelMap)
	if err != nil {
		return mappingJSON
	}
	return string(data)
}
