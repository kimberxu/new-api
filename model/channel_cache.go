package model

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	channelslowstream "github.com/QuantumNous/new-api/pkg/channel_slowstream"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
// channel2advancedCustomConfig caches parsed Advanced Custom (type 58) configs so
// path-aware selection avoids re-parsing JSON per request. Refreshed on full sync.
var channel2advancedCustomConfig map[int]*kitdto.AdvancedCustomConfig
var channelSyncLock sync.RWMutex

// [personal] modelGroupItemOverride holds member-level priority/weight
// overrides (nil = inherit the channel value). Keyed by
// modelGroupItemOverrides[groupName][model][channelId]; the groupName is the
// routable model name, model is the member's upstream model on that channel.
type modelGroupItemOverride struct {
	priority *int64
	weight   *uint
	// model is the member's real upstream model; empty means it equals the
	// group name (auto groups), so no upstream rewrite is needed.
	model string
}

// [personal] modelGroupItemOverrides caches member-level overrides for the
// routing selector (built in InitChannelCache).
var modelGroupItemOverrides = map[string]map[string]map[int]modelGroupItemOverride{}

// [personal] modelGroupParamOverride caches parsed group-level param
// override JSON (groupName → object). Invalid JSON is dropped at load.
var modelGroupParamOverride = map[string]map[string]interface{}{}

func InitChannelCache() {
	if !common.MemoryCacheEnabled {
		InvalidatePricingCache()
		rebuildTaskAliasView()
		return
	}
	// [personal] Reconcile every channel's models into model groups before
	// building the routing index from group members. Idempotent; this also
	// migrates pre-existing channels on first startup.
	if err := SyncAllModelGroups(); err != nil {
		common.SysLog(fmt.Sprintf("failed to sync all model groups: %v", err))
	}
	newChannelId2channel := make(map[int]*Channel)
	newChannel2advancedCustomConfig := make(map[int]*kitdto.AdvancedCustomConfig)
	var channels []*Channel
	DB.Find(&channels)
	// Load model-level disable records so disabled (channel, model) pairs are
	// excluded from the routing index.
	var disabledRows []ChannelDisabledModel
	DB.Find(&disabledRows)
	disabledByChannel := make(map[int]map[string]struct{}, len(disabledRows))
	for _, r := range disabledRows {
		if disabledByChannel[r.ChannelId] == nil {
			disabledByChannel[r.ChannelId] = make(map[string]struct{})
		}
		disabledByChannel[r.ChannelId][r.Model] = struct{}{}
	}
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
		if channel.Type == constant.ChannelTypeAdvancedCustom {
			if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
				newChannel2advancedCustomConfig[channel.Id] = config
			}
		}
	}
	// [personal] The routing index is built from model groups (group name =
	// routable model name) instead of the raw channel Models list: enabled
	// groups x enabled members on enabled channels become routable pairs.
	// Member-level priority/weight overrides are cached alongside for the
	// selector; the group-level param override JSON is parsed here too.
	enabledGroups, groupErr := GetEnabledModelGroupsWithItems()
	if groupErr != nil {
		common.SysError(fmt.Sprintf("failed to load enabled model groups: %v", groupErr))
		enabledGroups = map[string][]ModelGroupItem{}
	}
	newModelGroupItemOverrides := make(map[string]map[string]map[int]modelGroupItemOverride)
	newModelGroupParamOverride := make(map[string]map[string]interface{})
	allGroups, groupsErr := ListModelGroups("")
	if groupsErr == nil {
		for _, g := range allGroups {
			if strings.TrimSpace(g.ParamOverride) == "" {
				continue
			}
			var parsed map[string]interface{}
			if err := common.Unmarshal([]byte(g.ParamOverride), &parsed); err != nil {
				common.SysError(fmt.Sprintf("invalid param_override for model group %q: %v", g.Name, err))
				continue
			}
			newModelGroupParamOverride[g.Name] = parsed
		}
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	for groupName, items := range enabledGroups {
		for _, item := range items {
			channel, ok := newChannelId2channel[item.ChannelId]
			if !ok || channel.Status != common.ChannelStatusEnabled {
				continue // channel missing or disabled
			}
			if _, disabled := disabledByChannel[item.ChannelId][item.Model]; disabled {
				continue // (channel, model) model-level disabled
			}
			if newModelGroupItemOverrides[groupName] == nil {
				newModelGroupItemOverrides[groupName] = make(map[string]map[int]modelGroupItemOverride)
			}
			if newModelGroupItemOverrides[groupName][item.Model] == nil {
				newModelGroupItemOverrides[groupName][item.Model] = make(map[int]modelGroupItemOverride)
			}
			newModelGroupItemOverrides[groupName][item.Model][item.ChannelId] = modelGroupItemOverride{
				priority: item.Priority,
				weight:   item.Weight,
				model:    item.Model,
			}
			groups := strings.Split(channel.Group, ",")
			for _, group := range groups {
				if _, ok := newGroup2model2channels[group]; !ok {
					newGroup2model2channels[group] = make(map[string][]int)
				}
				if _, ok := newGroup2model2channels[group][groupName]; !ok {
					newGroup2model2channels[group][groupName] = make([]int, 0)
				}
				newGroup2model2channels[group][groupName] = append(newGroup2model2channels[group][groupName], channel.Id)
			}
		}
	}

	// sort by (effective) priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return effectivePriorityWith(channels[i], model, newChannelId2channel, newModelGroupItemOverrides) >
					effectivePriorityWith(channels[j], model, newChannelId2channel, newModelGroupItemOverrides)
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	modelGroupItemOverrides = newModelGroupItemOverrides
	modelGroupParamOverride = newModelGroupParamOverride
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	channel2advancedCustomConfig = newChannel2advancedCustomConfig
	channelSyncLock.Unlock()
	// Lock ordering: InvalidatePricingCache acquires updatePricingLock, and
	// GetPricing (holding updatePricingLock) nests channelSyncLock.RLock via
	// loadPricingAdvancedCustomConfigs. channelSyncLock MUST be released before
	// invalidating the pricing cache, otherwise the reversed order deadlocks.
	InvalidatePricingCache()
	rebuildTaskAliasView()
	common.SysLog("channels synced from database")
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

func GetRandomSatisfiedChannel(
	group string,
	model string,
	retry int,
	filters []dto.ChannelFilter,
	excludeChannels []int,
) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database.
	// [personal] The DB fallback is driven by model groups (not abilities).
	// It drops already-tried channels, targets the highest effective
	// priority tier, and draws weighted within the tier.
	if !common.MemoryCacheEnabled {
return GetRandomSatisfiedChannelFromGroups(group, model, excludeChannels)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// First, try to find channels with the exact model name.
	channels, _ := filterCandidateIDs(group2model2channels[group][model], model, filters)

	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channels, _ = filterCandidateIDs(group2model2channels[group][normalizedModel], model, filters)
	}

	if len(channels) == 0 {
		return nil, nil
	}

	// Build excluded channel id set for O(1) lookup. excludeChannels holds the
	// ids of channels already tried in the current request's retry loop. The
	// remaining channels are filtered to the highest priority tier, so retries
	// re-roll within the same tier until it is exhausted; only then does the
	// caller (auto-groups) cascade to a lower tier or lower group.
	excludeSet := make(map[int]bool, len(excludeChannels))
	for _, id := range excludeChannels {
		excludeSet[id] = true
	}
	var filteredChannels []int
	for _, id := range channels {
		if !excludeSet[id] {
			filteredChannels = append(filteredChannels, id)
		}
	}
	channels = filteredChannels

	if len(channels) == 0 {
		return nil, nil
	}

	if len(channels) == 1 {
		if channel, ok := channelsIDM[channels[0]]; ok {
			return channel, nil
		}
		return nil, fmt.Errorf("数据库一致性错误,渠道# %d 不存在,请联系管理员修复", channels[0])
	}

	// Pick the highest priority among the remaining (non-excluded) channels.
	// retry no longer indexes into a sorted priority list: the same tier keeps
	// being re-rolled until its channels are exhausted via excludeChannels.
	highestPriority := int64(0)
	first := true
	for _, channelId := range channels {
		if _, ok := channelsIDM[channelId]; !ok {
			return nil, fmt.Errorf("数据库一致性错误,渠道# %d 不存在,请联系管理员修复", channelId)
		}
		// [personal] member-level override wins, else channel priority
		priority := effectivePriority(channelId, model)
		// [deploy 分支定制] 慢速渠道降级：priority 拍平
		if demoted, p := channelslowstream.GetDemotedPriority(channelId, model, priority); demoted {
			priority = p
		}
		if first || priority > highestPriority {
			highestPriority = priority
			first = false
		}
	}

	// get the channels for the highest priority
	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			// [personal] member-level override wins, else channel priority
			priority := effectivePriority(channelId, model)
			// [deploy 分支定制] 慢速渠道降级：priority 拍平
			if demoted, p := channelslowstream.GetDemotedPriority(channelId, model, priority); demoted {
				priority = p
			}
			if priority == highestPriority {
				sumWeight += effectiveWeight(channelId, model)
				targetChannels = append(targetChannels, channel)
			}
		} else {
			return nil, fmt.Errorf("数据库一致性错误,渠道# %d 不存在,请联系管理员修复", channelId)
		}
	}

	if len(targetChannels) == 0 {
		return nil, errors.New(fmt.Sprintf("no channel found, group: %s, model: %s, priority: %d", group, model, highestPriority))
	}

	// smoothing factor and adjustment
	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		// when all channels have weight 0, set sumWeight to the number of channels and set smoothing adjustment to 100
		// each channel's effective weight = 100
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		// when the average weight is less than 10, set smoothing factor to 100
		smoothingFactor = 100
	}

	// Calculate the total weight of all channels up to endIdx
	totalWeight := sumWeight * smoothingFactor

	// Generate a random value in the range [0, totalWeight)
	randomWeight := rand.Intn(totalWeight)

	// Find a channel based on its weight
	for _, channel := range targetChannels {
		randomWeight -= effectiveWeight(channel.Id, model)*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	// return null if no channel is not found
	return nil, errors.New("channel not found")
}

// [personal] effectivePriorityWith resolves the effective routing priority for
// (channelId, model) inside group: a member-level override wins, else the
// channel's own priority. The overrides map is passed in so both the cache
// build (before the global swap) and the selector can share the logic.
func effectivePriorityWith(channelId int, model string, chanById map[int]*Channel, overrides map[string]map[string]map[int]modelGroupItemOverride) int64 {
	if groupModelMap, ok := overrides[model]; ok {
		if modelChanMap, ok := groupModelMap[model]; ok {
			if o, ok := modelChanMap[channelId]; ok && o.priority != nil {
				return *o.priority
			}
		}
		// Fall back: a manual group may hold a member whose model differs from
		// the group name (routable name). Any member override of this channel
		// in the group applies.
		for _, modelChanMap := range groupModelMap {
			if o, ok := modelChanMap[channelId]; ok && o.priority != nil {
				return *o.priority
			}
		}
	}
	if ch, ok := chanById[channelId]; ok {
		return ch.GetPriority()
	}
	return 0
}

// [personal] effectivePriority is the selector-time wrapper using the live
// global caches.
func effectivePriority(channelId int, model string) int64 {
	return effectivePriorityWith(channelId, model, channelsIDM, modelGroupItemOverrides)
}

// [personal] effectiveWeightWith mirrors effectivePriorityWith for weights.
func effectiveWeightWith(channelId int, model string, chanById map[int]*Channel, overrides map[string]map[string]map[int]modelGroupItemOverride) int {
	if groupModelMap, ok := overrides[model]; ok {
		if modelChanMap, ok := groupModelMap[model]; ok {
			if o, ok := modelChanMap[channelId]; ok && o.weight != nil {
				return int(*o.weight)
			}
		}
		for _, modelChanMap := range groupModelMap {
			if o, ok := modelChanMap[channelId]; ok && o.weight != nil {
				return int(*o.weight)
			}
		}
	}
	if ch, ok := chanById[channelId]; ok {
		return ch.GetWeight()
	}
	return 0
}

func effectiveWeight(channelId int, model string) int {
	return effectiveWeightWith(channelId, model, channelsIDM, modelGroupItemOverrides)
}

// [personal] GetModelGroupParamOverride returns the parsed group-level param
// override for the given routable model name, or nil when absent. Callers must
// treat the returned map as read-only.
func GetModelGroupParamOverride(groupName string) map[string]interface{} {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	return modelGroupParamOverride[groupName]
}

// filterChannelsByRequestPathAndModel restricts candidates by request path and
// model. Only Advanced Custom (type 58) channels are path-checked: they are kept
// only when one of their configured routes matches requestPath and model. All
// other channel types always pass. When requestPath is empty, filtering is skipped.
// Caller must hold channelSyncLock (read lock). The cached slice is never mutated.
func filterChannelsByRequestPathAndModel(channels []int, requestPath string, model string) []int {
	if requestPath == "" || len(channels) == 0 {
		return channels
	}
	filtered := make([]int, 0, len(channels))
	for _, channelId := range channels {
		channel, ok := channelsIDM[channelId]
		if !ok {
			// keep it so the downstream consistency error is raised as before
			filtered = append(filtered, channelId)
			continue
		}
		if channel.Type != constant.ChannelTypeAdvancedCustom {
			filtered = append(filtered, channelId)
			continue
		}
		if config := channel2advancedCustomConfig[channelId]; config != nil && config.SupportsPathForModel(requestPath, model) {
			filtered = append(filtered, channelId)
		}
	}
	return filtered
}
func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel, ok := channelsIDM[id]; ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// delete the channel from group2model2channels
		for group, model2channels := range group2model2channels {
			for model, channels := range model2channels {
				for i, channelId := range channels {
					if channelId == id {
						// remove the channel from the slice
						group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
						break
					}
				}
			}
		}
	}
}

// GetChannelIDsForGroupModel returns all enabled channel IDs for the given group
// and model from the memory cache. Returns nil when cache is disabled or no
// channels are found.
func GetChannelIDsForGroupModel(group string, modelName string) []int {
	if !common.MemoryCacheEnabled {
		return nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	modelMap, ok := group2model2channels[group]
	if !ok {
		return nil
	}
	ids, ok := modelMap[modelName]
	if !ok {
		normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
		ids, ok = modelMap[normalizedModel]
		if !ok {
			return nil
		}
	}
	if len(ids) == 0 {
		return nil
	}
	result := make([]int, len(ids))
	copy(result, ids)
	return result
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	if channel == nil {
		channelSyncLock.Unlock()
		return
	}

	if channelsIDM == nil {
		channelsIDM = make(map[int]*Channel)
	}
	if oldChannel, ok := channelsIDM[channel.Id]; ok {
		logger.LogDebug(nil, "CacheUpdateChannel before: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, oldChannel.ChannelInfo.MultiKeyPollingIndex)
	}
	channelsIDM[channel.Id] = channel
	if channel2advancedCustomConfig == nil {
		channel2advancedCustomConfig = make(map[int]*kitdto.AdvancedCustomConfig)
	}
	delete(channel2advancedCustomConfig, channel.Id)
	if channel.Type == constant.ChannelTypeAdvancedCustom {
		if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
			channel2advancedCustomConfig[channel.Id] = config
		}
	}
	logger.LogDebug(nil, "CacheUpdateChannel after: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)
	// Lock ordering: do NOT hold channelSyncLock while calling
	// InvalidatePricingCache. GetPricing acquires updatePricingLock first and then
	// channelSyncLock.RLock (via loadPricingAdvancedCustomConfigs); acquiring
	// updatePricingLock while holding channelSyncLock would be an AB-BA deadlock.
	channelSyncLock.Unlock()
	InvalidatePricingCache()
}
