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
	// [personal] SourceGroup marks a member that came from a referenced group
	// (empty = direct member of this group, non-empty = referenced group name).
	SourceGroup string `json:"source_group,omitempty"`
	ChannelPrio  int64  `json:"channel_priority"`
	ChannelWt    int    `json:"channel_weight"`
	// [personal] ChannelStatus is the live channel status (1 enabled,
	// 2 manually disabled, 3 auto disabled) so the UI can flag members that
	// are currently excluded from routing by their channel's state.
	ChannelStatus int `json:"channel_status"`
	// [personal] Disabled carries the model-level disable record (auto/manual)
	// for this (channel, model) pair, if any.
	Disabled *model.ChannelDisabledModel `json:"disabled,omitempty"`
}

// GroupView is a model group plus its member list.
type GroupView struct {
	*model.ModelGroup
	Members     []GroupMemberView     `json:"members,omitempty"`
	MemberCount int                   `json:"member_count"`
	References  []GroupReferenceView  `json:"references,omitempty"`
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
			Id:          it.Id,
			GroupId:     it.GroupId,
			ChannelId:   it.ChannelId,
			Model:       it.Model,
			Priority:    it.Priority,
			Weight:      it.Weight,
			Enabled:     it.Enabled,
			SourceGroup: it.SourceGroup,
		}
		if ch, err := model.GetChannelById(it.ChannelId, false); err == nil && ch != nil {
			view.ChannelName = ch.Name
			view.ChannelType = ch.Type
			view.ChannelPrio = ch.GetPriority()
			view.ChannelWt = ch.GetWeight()
			view.ChannelStatus = ch.Status
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
		items, err := model.ListModelGroupItemsExpanded(g.Id, 0, map[int]bool{})
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if withItems {
			v.Members = getGroupMemberViews(g.Id, items)
		}
		v.MemberCount = len(items)
		v.References = resolveGroupReferenceViews(g.Id)
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
	items, err := model.ListModelGroupItemsExpanded(g.Id, 0, map[int]bool{})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, GroupView{
		ModelGroup:  g,
		Members:     getGroupMemberViews(g.Id, items),
		MemberCount: len(items),
		References:  resolveGroupReferenceViews(g.Id),
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
	// [personal] Auto groups are system-managed: members are added/removed
	// only by channel init and the rebuild-routing action. Manual edits to an
	// auto group are limited to priority/weight/enabled (see UpdateGroupItem).
	if g.Source == model.GroupSourceAuto {
		common.ApiError(c, fmt.Errorf("auto group members are managed by the system; adjust priority/weight/enabled instead"))
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
	g, err := model.GetModelGroupByItemId(itemId)
	if err != nil || g == nil {
		common.ApiError(c, fmt.Errorf("group item #%d not found", itemId))
		return
	}
	if g.Source == model.GroupSourceAuto {
		common.ApiError(c, fmt.Errorf("auto group members are managed by the system; adjust priority/weight/enabled instead"))
		return
	}
	if err := model.DeleteModelGroupItem(itemId); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"deleted": true})
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
// GroupReferenceView is the API shape of a group reference with the target
// group's name resolved for display.
type GroupReferenceView struct {
	Id          int    `json:"id"`
	GroupId     int    `json:"group_id"`
	RefGroupId  int    `json:"ref_group_id"`
	RefGroupName string `json:"ref_group_name"`
	CreatedAt   int64  `json:"created_at"`
}

// resolveGroupReferenceViews loads reference rows for a group and resolves
// the referenced group names for display.
func resolveGroupReferenceViews(groupId int) []GroupReferenceView {
	refs, err := model.ListModelGroupReferences(groupId)
	if err != nil {
		return nil
	}
	views := make([]GroupReferenceView, 0, len(refs))
	for _, ref := range refs {
		v := GroupReferenceView{
			Id:         ref.Id,
			GroupId:    ref.GroupId,
			RefGroupId: ref.RefGroupId,
			CreatedAt:  ref.CreatedAt,
		}
		if g, err := model.GetModelGroupById(ref.RefGroupId); err == nil && g != nil {
			v.RefGroupName = g.Name
		}
		views = append(views, v)
	}
	return views
}

// ListGroupReferences returns the groups referenced by a model group.
func ListGroupReferences(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid group id"))
		return
	}
	views := resolveGroupReferenceViews(id)
	common.ApiSuccess(c, gin.H{"items": views, "total": len(views)})
}

// AddGroupReference makes a group include all members of another group.
func AddGroupReference(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid group id"))
		return
	}
	var req struct {
		RefGroupId int `json:"ref_group_id"`
	}
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.RefGroupId <= 0 {
		common.ApiError(c, fmt.Errorf("ref_group_id is required"))
		return
	}
	if err := model.AddModelGroupReference(id, req.RefGroupId); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"added": true})
}

// DeleteGroupReference removes a group reference.
func DeleteGroupReference(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid group id"))
		return
	}
	refGroupId, err := strconv.Atoi(c.Param("refGroupId"))
	if err != nil || refGroupId <= 0 {
		common.ApiError(c, fmt.Errorf("invalid reference group id"))
		return
	}
	if err := model.DeleteModelGroupReference(id, refGroupId); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"deleted": true})
}
