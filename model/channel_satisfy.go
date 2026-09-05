package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func IsChannelEnabledForGroupModel(group string, modelName string, channelID int) bool {
	if group == "" || modelName == "" || channelID <= 0 {
		return false
	}
	if !common.MemoryCacheEnabled {
		return isChannelEnabledForGroupModelDB(group, modelName, channelID)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return false
	}

	if isChannelIDInList(group2model2channels[group][modelName], channelID) {
		return true
	}
	normalized := ratio_setting.RoutingMatchModelName(modelName)
	if normalized != "" && normalized != modelName {
		return isChannelIDInList(group2model2channels[group][normalized], channelID)
	}
	return false
}

func IsChannelEnabledForAnyGroupModel(groups []string, modelName string, channelID int) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if IsChannelEnabledForGroupModel(g, modelName, channelID) {
			return true
		}
	}
	return false
}

func isChannelEnabledForGroupModelDB(group string, modelName string, channelID int) bool {
	probe := func(routable string) bool {
		var count int64
		err := DB.Table("model_group_items").
			Joins("JOIN model_groups ON model_group_items.group_id = model_groups.id").
			Joins("JOIN channels ON model_group_items.channel_id = channels.id").
			Where("model_groups.name = ? AND model_groups.enabled = ? AND model_group_items.enabled = ? AND channels.status = ? AND channels."+commonGroupCol+" = ? AND model_group_items.channel_id = ?",
				routable, true, true, common.ChannelStatusEnabled, group, channelID).
			Where("NOT EXISTS (SELECT 1 FROM channel_disabled_models WHERE channel_id = model_group_items.channel_id AND model = model_group_items.model)").
			Count(&count).Error
		return err == nil && count > 0
	}
	if probe(modelName) {
		return true
	}
	normalized := ratio_setting.RoutingMatchModelName(modelName)
	if normalized == "" || normalized == modelName {
		return false
	}
	return probe(normalized)
}

func isChannelIDInList(list []int, channelID int) bool {
	for _, id := range list {
		if id == channelID {
			return true
		}
	}
	return false
}
