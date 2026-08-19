package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// channelAbilityItem is one row of the channel×model route table. `id` is the
// row position within the filtered result (Ability has a composite key, no
// single id column); clients key rows by channel_id+model+group.
type channelAbilityItem struct {
	Id             int    `json:"id"`
	ChannelId      int    `json:"channel_id"`
	ChannelName    string `json:"channel_name"`
	ChannelType    int    `json:"channel_type"`
	Group          string `json:"group"`
	Model          string `json:"model"`
	Priority       *int64 `json:"priority"`
	Weight         uint   `json:"weight"`
	AbilityEnabled bool   `json:"ability_enabled"`
	Disabled       bool   `json:"disabled"`
	DisabledSource string `json:"disabled_source"`
	DisabledReason string `json:"disabled_reason"`
}

// GetChannelAbilities returns the paginated channel×model route table.
// Filtering by channel_id/model/group happens in SQL; the status filter and
// pagination happen in memory because status merges Ability.enabled with
// ChannelDisabledModel records (no JOIN, dialect-neutral).
func GetChannelAbilities(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	modelQuery := strings.TrimSpace(c.Query("model"))
	group := strings.TrimSpace(c.Query("group"))
	status := c.Query("status")
	if status == "" {
		status = "all"
	}

	abilities, err := model.GetChannelAbilities(channelId, modelQuery, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	channelIds := lo.Uniq(lo.Map(abilities, func(a model.Ability, _ int) int { return a.ChannelId }))
	channels, err := model.GetChannelsByIds(channelIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channelById := make(map[int]*model.Channel, len(channels))
	for _, ch := range channels {
		channelById[ch.Id] = ch
	}

	disabledModels, err := model.GetChannelDisabledModelsByChannelIds(channelIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// disabledByChannel[channelId][model] = records (manual preferred)
	disabledByChannel := make(map[int]map[string][]model.ChannelDisabledModel, len(disabledModels))
	for _, rec := range disabledModels {
		if disabledByChannel[rec.ChannelId] == nil {
			disabledByChannel[rec.ChannelId] = make(map[string][]model.ChannelDisabledModel)
		}
		disabledByChannel[rec.ChannelId][rec.Model] = append(disabledByChannel[rec.ChannelId][rec.Model], rec)
	}

	items := make([]channelAbilityItem, 0, len(abilities))
	for _, ability := range abilities {
		records := disabledByChannel[ability.ChannelId][ability.Model]
		var manualRec, autoRec *model.ChannelDisabledModel
		for i := range records {
			if records[i].Source == "manual" && manualRec == nil {
				manualRec = &records[i]
			} else if autoRec == nil {
				autoRec = &records[i]
			}
		}
		disabled := manualRec != nil || autoRec != nil
		disabledSource := ""
		disabledReason := ""
		if manualRec != nil {
			disabledSource = "manual"
			disabledReason = manualRec.Reason
		} else if autoRec != nil {
			disabledSource = "auto"
			disabledReason = autoRec.Reason
		}

		// status filter (in memory)
		switch status {
		case "enabled":
			if !ability.Enabled || disabled {
				continue
			}
		case "manual_disabled":
			if manualRec == nil {
				continue
			}
		case "auto_disabled":
			if manualRec != nil || autoRec == nil {
				continue
			}
		}

		ch := channelById[ability.ChannelId]
		channelName := ""
		channelType := 0
		if ch != nil {
			channelName = ch.Name
			channelType = ch.Type
		}
		items = append(items, channelAbilityItem{
			Id:             len(items) + 1,
			ChannelId:      ability.ChannelId,
			ChannelName:    channelName,
			ChannelType:    channelType,
			Group:          ability.Group,
			Model:          ability.Model,
			Priority:       ability.Priority,
			Weight:         ability.Weight,
			AbilityEnabled: ability.Enabled,
			Disabled:       disabled,
			DisabledSource: disabledSource,
			DisabledReason: disabledReason,
		})
	}

	total := len(items)
	start := pageInfo.GetStartIdx()
	if start > total {
		start = total
	}
	end := pageInfo.GetEndIdx()
	if end > total {
		end = total
	}
	pageItems := items[start:end]

	common.ApiSuccess(c, gin.H{
		"items":     pageItems,
		"total":     total,
		"page":      pageInfo.GetPage(),
		"page_size": pageInfo.GetPageSize(),
	})
}

type DisableChannelAbilitiesRequest struct {
	ChannelId int      `json:"channel_id"`
	Models    []string `json:"models"`
	Reason    string   `json:"reason"`
}

// DisableChannelAbilities manually disables specific models on a channel.
func DisableChannelAbilities(c *gin.Context) {
	var req DisableChannelAbilitiesRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.ChannelId <= 0 {
		common.ApiError(c, fmt.Errorf("channel_id is required"))
		return
	}
	models := lo.Uniq(lo.FilterMap(req.Models, func(m string, _ int) (string, bool) {
		m = strings.TrimSpace(m)
		return m, m != ""
	}))
	if len(models) == 0 {
		common.ApiError(c, fmt.Errorf("models is required"))
		return
	}
	if _, err := model.GetChannelById(req.ChannelId, false); err != nil {
		common.ApiError(c, fmt.Errorf("channel #%d not found", req.ChannelId))
		return
	}

	if err := model.AddChannelDisabledModels(req.ChannelId, models, "manual", req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"disabled": len(models)})
}

type EnableChannelAbilitiesRequest struct {
	ChannelId int      `json:"channel_id"`
	Models    []string `json:"models"`
}

// EnableChannelAbilities re-enables specific models on a channel (any source).
func EnableChannelAbilities(c *gin.Context) {
	var req EnableChannelAbilitiesRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.ChannelId <= 0 {
		common.ApiError(c, fmt.Errorf("channel_id is required"))
		return
	}
	models := lo.Uniq(lo.FilterMap(req.Models, func(m string, _ int) (string, bool) {
		m = strings.TrimSpace(m)
		return m, m != ""
	}))
	if len(models) == 0 {
		common.ApiError(c, fmt.Errorf("models is required"))
		return
	}

	if err := model.DeleteChannelDisabledModels(req.ChannelId, models); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"enabled": len(models)})
}
