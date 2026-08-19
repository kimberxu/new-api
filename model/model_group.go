package model

import (
	"errors"
	"fmt"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GroupSourceAuto / GroupSourceManual identify how a model group came to be.
const (
	GroupSourceAuto   = "auto"
	GroupSourceManual = "manual"
)

// ModelGroup is a route-table group: its Name doubles as the routable model
// name (request model X → group X → members selected by item priority/weight).
type ModelGroup struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	Name      string `json:"name" gorm:"type:varchar(128);uniqueIndex"`
	Source    string `json:"source" gorm:"type:varchar(16)"` // auto | manual；默认值代码层设置
	Enabled   bool   `json:"enabled"`                        // 组级开关；默认 true 代码层设置
	// ParamOverride is a group-level parameter override JSON (same schema as
	// the channel-level param_override). Empty string = no override.
	ParamOverride string `json:"param_override" gorm:"type:text"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;autoCreateTime"`
}

// ModelGroupItem is one member of a model group: a real upstream model on a
// concrete channel, with group-level priority/weight (inherited from the
// channel on auto-sync, overridable per member).
type ModelGroupItem struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	GroupId   int    `json:"group_id" gorm:"index;uniqueIndex:idx_group_channel_model"`
	ChannelId int    `json:"channel_id" gorm:"uniqueIndex:idx_group_channel_model"`
	Model     string `json:"model" gorm:"type:varchar(128);uniqueIndex:idx_group_channel_model"`
	// Priority/Weight are pointers: nil = inherit channel value at read time,
	// non-nil = explicit override that auto-sync must not rewrite.
	Priority  *int64 `json:"priority" gorm:"bigint;default:0"`
	Weight    *uint  `json:"weight" gorm:"default:0"`
	Enabled   bool   `json:"enabled"` // 组内成员级启用；默认 true 代码层设置
	CreatedAt int64  `json:"created_at" gorm:"bigint;autoCreateTime"`
}

func (g *ModelGroup) SetDefaults() {
	if g.Source == "" {
		g.Source = GroupSourceAuto
	}
	if !g.Enabled && g.Id == 0 {
		g.Enabled = true
	}
}

func (i *ModelGroupItem) SetDefaults() {
	if !i.Enabled && i.Id == 0 {
		i.Enabled = true
	}
}

// CreateModelGroup inserts a manual group. A group with the same name (auto or
// manual) is rejected via a 409-style error message.
func CreateModelGroup(name string, source string) (*ModelGroup, error) {
	if source == "" {
		source = GroupSourceManual
	}
	group := &ModelGroup{Name: name, Source: source, Enabled: true}
	group.SetDefaults()
	err := DB.Create(group).Error
	if err != nil {
		return nil, fmt.Errorf("failed to create model group %q: %w", name, err)
	}
	return group, nil
}

// GetModelGroupByName returns the group for the given name, or nil when absent.
func GetModelGroupByName(name string) (*ModelGroup, error) {
	var g ModelGroup
	err := DB.Where("name = ?", name).First(&g).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}

// GetModelGroupById returns a group by id.
func GetModelGroupById(id int) (*ModelGroup, error) {
	var g ModelGroup
	err := DB.First(&g, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}

// GetModelGroupByItemId resolves the owning group of a member item.
func GetModelGroupByItemId(itemId int) (*ModelGroup, error) {
	var item ModelGroupItem
	if err := DB.First(&item, itemId).Error; err != nil {
		return nil, err
	}
	return GetModelGroupById(item.GroupId)
}

// ListModelGroups returns all groups (optionally only one source).
func ListModelGroups(source string) ([]*ModelGroup, error) {
	var groups []*ModelGroup
	q := DB
	if source != "" {
		q = q.Where("source = ?", source)
	}
	err := q.Order("name asc").Find(&groups).Error
	return groups, err
}

// ListModelGroupItems returns all members for a group.
func ListModelGroupItems(groupId int) ([]*ModelGroupItem, error) {
	var items []*ModelGroupItem
	err := DB.Where("group_id = ?", groupId).Order("channel_id asc").Find(&items).Error
	return items, err
}

// GetAllModelGroupItems returns every member (cache building).
func GetAllModelGroupItems() ([]*ModelGroupItem, error) {
	var items []*ModelGroupItem
	err := DB.Find(&items).Error
	return items, err
}

// SetModelGroupEnabled toggles a group-level switch.
func SetModelGroupEnabled(groupId int, enabled bool) error {
	return DB.Model(&ModelGroup{}).Where("id = ?", groupId).Update("enabled", enabled).Error
}

// DeleteModelGroup removes a group and its members in one transaction.
func DeleteModelGroup(groupId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupId).Delete(&ModelGroupItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&ModelGroup{}, groupId).Error
	})
}

// AddModelGroupItems inserts members, ignoring duplicates. New items inherit
// the passed priority/weight (caller resolves channel values for auto-sync or
// explicit overrides for manual adds).
func AddModelGroupItems(groupId int, items []ModelGroupItem) error {
	for i := range items {
		items[i].GroupId = groupId
		items[i].SetDefaults()
	}
	if len(items) == 0 {
		return nil
	}
	for _, chunk := range lo.Chunk(items, 50) {
		err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateModelGroupItem applies a partial update to a member (enabled /
// priority / weight). Only non-nil fields are written so callers can patch
// incrementally; enabled uses a pointer to distinguish "not touched".
func UpdateModelGroupItem(itemId int, enabled *bool, priority *int64, weight *uint) error {
	updates := map[string]interface{}{}
	if enabled != nil {
		updates["enabled"] = *enabled
	}
	if priority != nil {
		updates["priority"] = *priority
	}
	if weight != nil {
		updates["weight"] = *weight
	}
	if len(updates) == 0 {
		return nil
	}
	return DB.Model(&ModelGroupItem{}).Where("id = ?", itemId).Updates(updates).Error
}

// DeleteModelGroupItem removes one member.
func DeleteModelGroupItem(itemId int) error {
	return DB.Delete(&ModelGroupItem{}, itemId).Error
}

// DeleteModelGroupItemsByChannelId removes members belonging to a channel
// (channel deletion cleanup).
func DeleteModelGroupItemsByChannelId(channelId int) error {
	return DB.Where("channel_id = ?", channelId).Delete(&ModelGroupItem{}).Error
}

// DeleteModelGroupItemsByChannelIds removes members for many channels (bulk
// channel deletion cleanup).
func DeleteModelGroupItemsByChannelIds(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return DB.Where("channel_id IN ?", ids).Delete(&ModelGroupItem{}).Error
}

// DeleteModelGroupItemsByDisabledChannelStatus removes members of channels
// that are currently disabled (DeleteDisabledChannel cleanup). Runs before
// the channels are deleted, matching the channel_disabled_models pattern.
func DeleteModelGroupItemsByDisabledChannelStatus(statuses ...int) error {
	if len(statuses) == 0 {
		return nil
	}
	return DB.Where("channel_id IN (SELECT id FROM channels WHERE status IN ?)", statuses).
		Delete(&ModelGroupItem{}).Error
}

// SetModelGroupParamOverride updates the group-level parameter override JSON.
// An empty string clears the override.
func SetModelGroupParamOverride(groupId int, paramOverride string) error {
	return DB.Model(&ModelGroup{}).Where("id = ?", groupId).Update("param_override", paramOverride).Error
}

// GetModelGroupByNameMap returns the groups for the given names keyed by name.
// Missing groups are simply absent from the map.
func GetModelGroupByNameMap(names []string) (map[string]*ModelGroup, error) {
	result := make(map[string]*ModelGroup, len(names))
	if len(names) == 0 {
		return result, nil
	}
	var groups []*ModelGroup
	if err := DB.Where("name IN ?", names).Find(&groups).Error; err != nil {
		return nil, err
	}
	for _, g := range groups {
		result[g.Name] = g
	}
	return result, nil
}

// DeleteModelGroupItemsNotIn removes all members of a channel whose model is
// no longer in the given list. An empty list deletes every member of the
// channel (the channel has no routable models left).
func DeleteModelGroupItemsNotIn(channelId int, models []string) error {
	q := DB.Where("channel_id = ?", channelId)
	if len(models) == 0 {
		// models empty: delete all members of this channel
		return q.Delete(&ModelGroupItem{}).Error
	}
	return q.Where("model NOT IN ?", models).Delete(&ModelGroupItem{}).Error
}

// GetEnabledModelGroupsWithItems returns enabled groups mapped to their
// enabled members (route-cache building). Full-table scan; fine at personal
// scale.
func GetEnabledModelGroupsWithItems() (map[string][]ModelGroupItem, error) {
	var groups []*ModelGroup
	if err := DB.Where("enabled = ?", true).Find(&groups).Error; err != nil {
		return nil, err
	}
	var items []ModelGroupItem
	if err := DB.Where("enabled = ?", true).Find(&items).Error; err != nil {
		return nil, err
	}
	itemByGroup := make(map[int][]ModelGroupItem, len(groups))
	for _, it := range items {
		itemByGroup[it.GroupId] = append(itemByGroup[it.GroupId], it)
	}
	result := make(map[string][]ModelGroupItem, len(groups))
	for _, g := range groups {
		result[g.Name] = itemByGroup[g.Id]
	}
	return result, nil
}