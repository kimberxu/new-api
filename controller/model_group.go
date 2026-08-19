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

// GroupMemberView is the API shape of a model-group member with resolved
// channel info and effective (inherited or overridden) priority/weight.
type GroupMemberView struct {
	Id           int    `json:"id"`
	GroupId      int    `json:"group_id"`
	ChannelId    int    `json:"channel_id"`
	ChannelName  string `json:"channel_name"`
	ChannelType  int    `json:"channel_type"`
	Model        string `json:"model"`
	Priority     *int64 `json:"priority"`
	Weight       *uint  `json:"weight"`
	Enabled      bool   `json:"enabled"`
	ChannelPrio  int64  `json:"channel_priority"`
	ChannelWt    int    `json:"channel_weight"`
	// [personal] Disabled carries the model-level disable record (auto/manual)
	// for this (channel, model) pair, if any.
	Disabled *model.ChannelDisabledModel `json:"disabled,omitempty"`
}

// GroupView is a model group plus its member list.
type GroupView struct {
	*model.ModelGroup
	Members []GroupMemberView `json:"members,omitempty"`
	MemberCount int `json:"member_count"`
}

// getGroupMemberViews resolves members with channel metadata.
func getGroupMemberViews(groupId int, items []*model.ModelGroupItem) []GroupMemberView {
	views := make([]GroupMemberView, 0, len(items))
	if len(items) == 0 {
		return views
	}
	// [personal] Batch-load model-level disable records for the involved
	// channels (channel_disabled_models) so members can surface their ban
	// status without a query per member.
	channelIds := make([]int, 0, len(items))
	for _, it := range items {
		channelIds = append(channelIds, it.ChannelId)
	}
	disabledRows, _ := model.GetChannelDisabledModelsByChannelIds(channelIds)
	disabledByChannelModel := make(map[int]map[string]*model.ChannelDisabledModel)
	for i := range disabledRows {
		r := &disabledRows[i]
		if disabledByChannelModel[r.ChannelId] == nil {
			disabledByChannelModel[r.ChannelId] = make(map[string]*model.ChannelDisabledModel)
		}
		disabledByChannelModel[r.ChannelId][r.Model] = r
	}
	for _, it := range items {
		view := GroupMemberView{
			Id:        it.Id,
			GroupId:   it.GroupId,
			ChannelId: it.ChannelId,
			Model:     it.Model,
			Priority:  it.Priority,
			Weight:    it.Weight,
			Enabled:   it.Enabled,
		}
		if ch, err := model.GetChannelById(it.ChannelId, false); err == nil && ch != nil {
			view.ChannelName = ch.Name
			view.ChannelType = ch.Type
			view.ChannelPrio = ch.GetPriority()
			view.ChannelWt = ch.GetWeight()
		}
		if byModel, ok := disabledByChannelModel[it.ChannelId]; ok {
			view.Disabled = byModel[it.Model]
		}
		views = append(views, view)
	}
	return views
}

// ListModelGroups returns all groups (members included when ?with_items=1).
func ListModelGroups(c *gin.Context) {
	groups, err := model.ListModelGroups(c.Query("source"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	withItems := c.Query("with_items") == "1"
	views := make([]GroupView, 0, len(groups))
	for _, g := range groups {
		v := GroupView{ModelGroup: g}
		if withItems {
			items, err := model.ListModelGroupItems(g.Id)
			if err != nil {
				common.ApiError(c, err)
				return
			}
			v.Members = getGroupMemberViews(g.Id, items)
		}
		items, err := model.ListModelGroupItems(g.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		v.MemberCount = len(items)
		views = append(views, v)
	}
	common.ApiSuccess(c, gin.H{
		"items": views,
		"total": len(views),
	})
}

// GetModelGroup returns one group with members.
func GetModelGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid group id"))
		return
	}
	g, err := model.GetModelGroupById(id)
	if err != nil || g == nil {
		common.ApiError(c, fmt.Errorf("model group #%d not found", id))
		return
	}
	items, err := model.ListModelGroupItems(g.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, GroupView{
		ModelGroup:  g,
		Members:     getGroupMemberViews(g.Id, items),
		MemberCount: len(items),
	})
}

type CreateModelGroupRequest struct {
	Name string `json:"name"`
}

// CreateModelGroup creates a manual model group (group name = routable model
// name). Auto groups are created by the sync task and must not be duplicated
// here.
func CreateModelGroup(c *gin.Context) {
	var req CreateModelGroupRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		common.ApiError(c, fmt.Errorf("group name is required"))
		return
	}
	if strings.ContainsAny(name, ",; \t\n") {
		common.ApiError(c, fmt.Errorf("group name must not contain spaces, commas or semicolons"))
		return
	}
	if existing, err := model.GetModelGroupByName(name); err != nil {
		common.ApiError(c, err)
		return
	} else if existing != nil {
		common.ApiError(c, fmt.Errorf("model group %q already exists", name))
		return
	}
	g, err := model.CreateModelGroup(name, model.GroupSourceManual)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, g)
}

// DeleteModelGroup removes a manual group and its members. Auto groups are
// system-managed and rejected.
func DeleteModelGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid group id"))
		return
	}
	g, err := model.GetModelGroupById(id)
	if err != nil || g == nil {
		common.ApiError(c, fmt.Errorf("model group #%d not found", id))
		return
	}
	if g.Source == model.GroupSourceAuto {
		common.ApiError(c, fmt.Errorf("auto groups are system-managed and cannot be deleted"))
		return
	}
	if err := model.DeleteModelGroup(g.Id); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"deleted": true})
}

// SetModelGroupEnabled toggles a group-level switch for manual groups.
func SetModelGroupEnabled(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid group id"))
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SetModelGroupEnabled(id, req.Enabled); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"enabled": req.Enabled})
}

type AddGroupItemRequest struct {
	ChannelId int    `json:"channel_id"`
	Model     string `json:"model"`
	Priority  *int64 `json:"priority"`
	Weight    *uint  `json:"weight"`
}

// AddGroupItem adds a real upstream model of an existing channel to a group.
// Priority/weight default to the channel's own values (nil = inherit).
func AddGroupItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid group id"))
		return
	}
	g, err := model.GetModelGroupById(id)
	if err != nil || g == nil {
		common.ApiError(c, fmt.Errorf("model group #%d not found", id))
		return
	}
	var req AddGroupItemRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	if req.ChannelId <= 0 || req.Model == "" {
		common.ApiError(c, fmt.Errorf("channel_id and model are required"))
		return
	}
	// The model must really exist on the channel (real upstream model).
	ch, err := model.GetChannelById(req.ChannelId, false)
	if err != nil || ch == nil {
		common.ApiError(c, fmt.Errorf("channel #%d not found", req.ChannelId))
		return
	}
	if !lo.Contains(ch.GetModels(), req.Model) {
		common.ApiError(c, fmt.Errorf("model %q does not exist on channel #%d", req.Model, req.ChannelId))
		return
	}
	items := []model.ModelGroupItem{{
		ChannelId: req.ChannelId,
		Model:     req.Model,
		Priority:  req.Priority, // nil = inherit channel priority
		Weight:    req.Weight,   // nil = inherit channel weight
		Enabled:   true,
	}}
	if err := model.AddModelGroupItems(g.Id, items); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"added": 1})
}

type UpdateGroupItemRequest struct {
	Enabled  *bool  `json:"enabled"`
	Priority *int64 `json:"priority"`
	Weight   *uint  `json:"weight"`
}

// UpdateGroupItem patches a member's enabled / priority / weight.
func UpdateGroupItem(c *gin.Context) {
	itemId, err := strconv.Atoi(c.Param("itemId"))
	if err != nil || itemId <= 0 {
		common.ApiError(c, fmt.Errorf("invalid item id"))
		return
	}
	var req UpdateGroupItemRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Enabled == nil && req.Priority == nil && req.Weight == nil {
		common.ApiError(c, fmt.Errorf("nothing to update"))
		return
	}
	if err := model.UpdateModelGroupItem(itemId, req.Enabled, req.Priority, req.Weight); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"updated": true})
}

// DeleteGroupItem removes a member from its group.
func DeleteGroupItem(c *gin.Context) {
	itemId, err := strconv.Atoi(c.Param("itemId"))
	if err != nil || itemId <= 0 {
		common.ApiError(c, fmt.Errorf("invalid item id"))
		return
	}
	if err := model.DeleteModelGroupItem(itemId); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"deleted": true})
}

// GetChannelModelOptions lists which models of a channel could be added to
// groups — simply the channel's Models list (real upstream models).
func GetChannelModelOptions(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid channel id"))
		return
	}
	ch, err := model.GetChannelById(id, false)
	if err != nil || ch == nil {
		common.ApiError(c, fmt.Errorf("channel #%d not found", id))
		return
	}
	common.ApiSuccess(c, gin.H{"models": ch.GetModels()})
}

// SetModelGroupParamOverride validates the group-level param override JSON
// (same schema as the channel-level param_override) and stores it on the
// group. An empty string clears the override.
func SetModelGroupParamOverride(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid group id"))
		return
	}
	g, err := model.GetModelGroupById(id)
	if err != nil || g == nil {
		common.ApiError(c, fmt.Errorf("model group #%d not found", id))
		return
	}
	var req struct {
		ParamOverride string `json:"param_override"`
	}
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(req.ParamOverride) != "" {
		var parsed map[string]interface{}
		if err := common.Unmarshal([]byte(req.ParamOverride), &parsed); err != nil {
			common.ApiError(c, fmt.Errorf("param_override must be a valid JSON object"))
			return
		}
	}
	if err := model.SetModelGroupParamOverride(g.Id, req.ParamOverride); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"param_override": req.ParamOverride})
}