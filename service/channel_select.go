package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/gin-gonic/gin"
)

func GetChannelConstraints(c *gin.Context) *dto.ChannelConstraints {
	if c == nil {
		return &dto.ChannelConstraints{}
	}
	if existing, ok := common.GetContextKeyType[*dto.ChannelConstraints](c, constant.ContextKeyChannelConstraints); ok && existing != nil {
		return existing
	}
	constraints := &dto.ChannelConstraints{}
	common.SetContextKey(c, constant.ContextKeyChannelConstraints, constraints)
	return constraints
}

func AppendTaskPluginIdentityFilter(c *gin.Context, pluginKey string) {
	if c == nil {
		return
	}
	GetChannelConstraints(c).AddFilter(dto.ChannelFilter{
		Kind:                   dto.FilterTaskPluginIdentity,
		TaskPluginKey:          pluginKey,
		TaskPluginChannelTypes: pinnedTaskPluginChannelTypes(c, pluginKey),
	})
}

type RetryParam struct {
	Ctx             *gin.Context
	TokenGroup      string
	ModelName       string
	RequestPath     string
	Retry           *int
	ExcludeChannels []int
	resetNextTry    bool
}

func (p *RetryParam) ExcludeChannel(id int) {
	if p.ExcludeChannels == nil {
		p.ExcludeChannels = []int{}
	}
	p.ExcludeChannels = append(p.ExcludeChannels, id)
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

// CacheGetRandomSatisfiedChannel tries to get a random channel that satisfies the requirements.
// 尝试获取一个满足要求的随机渠道。
//
// Channel selection is exclude-driven: the caller accumulates the ids of
// already-tried (failed or rate-limited) channels in param.ExcludeChannels, and
// each call re-rolls only among the highest priority tier of the remaining
// channels. Retries therefore prefer another channel at the same priority;
// only when that tier is fully excluded does selection cascade to the next
// lower priority tier, and (for "auto" groups) to the next group.
// 渠道选择由排除集合驱动:调用方将已尝试(失败或限流)的渠道 id 累积在
// param.ExcludeChannels 中,每次调用只在剩余渠道的最高优先级层内重新随机。
// 因此重试会优先选择同一优先级的其他渠道;只有该层被全部排除后,选择才会
// 级联到下一个较低优先级层,以及(对于 "auto" 分组)下一个分组。
//
// For "auto" tokenGroup with cross-group Retry enabled:
// 对于启用了跨分组重试的 "auto" tokenGroup:
//
//   - Uses ContextKeyAutoGroupIndex to track current group index.
//     使用 ContextKeyAutoGroupIndex 跟踪当前分组索引。
//
//   - When GetRandomSatisfiedChannel returns nil for the current group, moves to next group.
//     当当前分组无可用渠道(GetRandomSatisfiedChannel 返回 nil)时,切换到下一个分组。
//
//   - Each group gets at most RetryTimes+1 attempts (same budget as the legacy
//     per-group priority indexing): a group is left after either its channels
//     are exhausted (nil result) or the pre-switch condition
//     (crossGroupRetry && priorityRetry >= RetryTimes) fires.
//     每个分组最多尝试 RetryTimes+1 次(与旧的按优先级索引语义预算一致):
//     分组在渠道全部排除(nil 结果)或满足预切换条件
//     (crossGroupRetry && priorityRetry >= RetryTimes)后切换。
//
// Example flow (2 groups, RetryTimes=3, exclude-driven re-roll within a tier):
// 示例流程(2个分组,RetryTimes=3,层内排除驱动重选):
//
//	Retry=0: GroupA, highest remaining priority tier
//	         分组A, 剩余最高优先级层
//
//	Retry=1: GroupA, same tier while any channel remains, else next tier
//	         分组A, 层内仍有渠道则同层,否则降级到下一层
//
//	Retry=2: GroupA exhausted (or pre-switch) → GroupB
//	         分组A用完(或预切换)→ 分组B
//
//	Retry=3: GroupB, highest remaining priority tier
//	         分组B, 剩余最高优先级层
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	var channel *model.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)
	filters := GetChannelConstraints(param.Ctx).Filters

	if param.TokenGroup == "auto" {
		autoGroups := GetRequestAutoGroups(param.Ctx, userGroup)
		if len(autoGroups) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}

		// startGroupIndex: the group index to start searching from
		// startGroupIndex: 开始搜索的分组索引
		startGroupIndex := 0
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)

		if lastGroupIndex, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			// Retry counter within the current group. It is passed to
			// GetRandomSatisfiedChannel only for signature compatibility;
			// selection itself is exclude-driven and no longer indexes a
			// priority list. It is still compared against RetryTimes for the
			// cross-group pre-switch below.
			priorityRetry := param.GetRetry()
			// If moved to a new group, reset priorityRetry
			// 如果切换到新分组,重置 priorityRetry
			if i > startGroupIndex {
				priorityRetry = 0
			}
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, priorityRetry: %d", autoGroup, priorityRetry)

			channel, _ = model.GetRandomSatisfiedChannel(
				autoGroup,
				param.ModelName,
				priorityRetry,
				filters,
				param.ExcludeChannels,
			)
			if channel == nil {
				// Current group has no available channel for this model, try next group
				// 当前分组没有该模型的可用渠道,尝试下一个分组
				logger.LogDebug(param.Ctx, "No available channel in group %s for model %s at priorityRetry %d, trying next group", autoGroup, param.ModelName, priorityRetry)
				// 重置状态以尝试下一个分组
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器,以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				continue
			}
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			selectGroup = autoGroup
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)

			// Prepare state for next retry
			// 为下一次重试准备状态
			if crossGroupRetry && priorityRetry >= common.RetryTimes {
				// Current group has exhausted all retries, prepare to switch to next group
				// This request still uses current group, but next retry will use next group
				// 当前分组已用完所有重试次数，准备切换到下一个分组
				// 本次请求仍使用当前分组，但下次重试将使用下一个分组
				logger.LogDebug(param.Ctx, "Current group %s retries exhausted (priorityRetry=%d >= RetryTimes=%d), preparing switch to next group for next retry", autoGroup, priorityRetry, common.RetryTimes)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				param.ResetRetryNextTry()
			} else {
				// Stay in current group, save current state
				// 保持在当前分组，保存当前状态
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			}
			break
		}
	} else {
		channel, err = model.GetRandomSatisfiedChannel(
			param.TokenGroup,
			param.ModelName,
			param.GetRetry(),
			filters,
			param.ExcludeChannels,
		)
		if err != nil {
			return nil, param.TokenGroup, err
		}
	}
	// [personal] Row-level passthrough: record the selected member's upstream
	// model so SetupContextForSelectedChannel and the ban path reuse it
	// instead of re-guessing among sibling members on the same channel.
	// ResolveModelGroupUpstreamModel is deterministic per (model, channel):
	// cache path reads the best member override, DB path takes the single
	// best row via ORDER BY … LIMIT 1 with the same
	// (priority → weight → model ASC) rule as the selectors' per-channel
	// aggregation, so no second random roll happens here.
	// param.ModelName is the routable model in both branches (auto or not).
	if channel != nil && param != nil && param.Ctx != nil && param.ModelName != "" {
		common.SetContextKey(param.Ctx, constant.ContextKeySelectedUpstreamModel, model.ResolveModelGroupUpstreamModel(param.ModelName, channel.Id))
	}
	return channel, selectGroup, nil
}

func pinnedTaskPluginChannelTypes(c *gin.Context, expected string) []int {
	if c == nil || expected == "" {
		return nil
	}
	if value, exists := c.Get(jsplugin.ContextKeyPinnedEndpoint); exists {
		pinned, ok := value.(jsplugin.PinnedEndpoint)
		if ok && pinned.Generation != nil && len(pinned.Candidates) > 1 {
			expectedFound := false
			channelTypes := make([]int, 0, len(pinned.Candidates))
			seen := make(map[int]struct{}, len(pinned.Candidates))
			for _, candidate := range pinned.Candidates {
				if candidate.Plugin == nil {
					continue
				}
				if candidate.Plugin.Meta.Key == expected {
					expectedFound = true
				}
				for _, channelType := range candidate.Plugin.Meta.ChannelTypes {
					if channelType == 0 || channelType == constant.ChannelTypeTaskPlugin {
						continue
					}
					if _, duplicate := seen[channelType]; duplicate {
						continue
					}
					if plugin, indexed := pinned.Generation.GetByChannelType(channelType); indexed && plugin == candidate.Plugin {
						seen[channelType] = struct{}{}
						channelTypes = append(channelTypes, channelType)
					}
				}
			}
			if expectedFound {
				return channelTypes
			}
		}
	}
	value, exists := c.Get(jsplugin.ContextKeyPinnedPlugin)
	pinned, ok := value.(jsplugin.PinnedPlugin)
	if !exists || !ok || pinned.Generation == nil || pinned.Plugin == nil || pinned.Plugin.Meta.Key != expected {
		return nil
	}
	channelTypes := make([]int, 0, len(pinned.Plugin.Meta.ChannelTypes))
	for _, channelType := range pinned.Plugin.Meta.ChannelTypes {
		if channelType == 0 || channelType == constant.ChannelTypeTaskPlugin {
			continue
		}
		channelTypes = append(channelTypes, channelType)
	}
	if len(channelTypes) == 0 {
		return nil
	}
	return channelTypes
}
