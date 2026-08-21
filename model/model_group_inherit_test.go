package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Regression: a member created with nil priority/weight must persist NULL
// (= inherit channel values at read time), not 0 (an explicit override).
// The gorm "default:0" tags used to backfill 0 and break inheritance.
func TestModelGroupItemNilPriorityWeightPersistedAsNull(t *testing.T) {
	group := ModelGroup{Name: "zz-inherit-test", Source: GroupSourceManual, Enabled: true}
	require.NoError(t, DB.Create(&group).Error)
	t.Cleanup(func() { DB.Where("name = ?", group.Name).Delete(&ModelGroup{}) })

	item := ModelGroupItem{GroupId: group.Id, ChannelId: 999001, Model: "zz-inherit-model", Enabled: true}
	require.NoError(t, DB.Create(&item).Error)
	t.Cleanup(func() { DB.Where("channel_id = ?", item.ChannelId).Delete(&ModelGroupItem{}) })
	require.Nil(t, item.Priority)
	require.Nil(t, item.Weight)

	var raw struct {
		Priority *int64
		Weight   *uint
	}
	require.NoError(t, DB.Table("model_group_items").
		Select("priority, weight").
		Where("channel_id = ?", item.ChannelId).
		Scan(&raw).Error)
	require.Nil(t, raw.Priority)
	require.Nil(t, raw.Weight)
}

// The one-time repair nulls legacy 0-valued priority/weight (written by the
// default:0 bug) while keeping non-zero overrides, and is a no-op afterwards.
func TestRepairModelGroupItemInheritance(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))
	t.Cleanup(func() { DB.Where("key = ?", "ModelGroupInheritRepairDone").Delete(&Option{}) })

	group := ModelGroup{Name: "zz-repair-test", Source: GroupSourceManual, Enabled: true}
	require.NoError(t, DB.Create(&group).Error)
	t.Cleanup(func() { DB.Where("name = ?", group.Name).Delete(&ModelGroup{}) })

	pZero, pNine := int64(0), int64(9)
	wZero := uint(0)
	items := []ModelGroupItem{
		{GroupId: group.Id, ChannelId: 999101, Model: "m", Priority: &pZero, Weight: &wZero, Enabled: true},
	}
	require.NoError(t, DB.Create(&items).Error)
	override := ModelGroupItem{GroupId: group.Id, ChannelId: 999102, Model: "m", Priority: &pNine, Enabled: true}
	require.NoError(t, DB.Create(&override).Error)
	t.Cleanup(func() {
		DB.Where("channel_id IN ?", []int{999101, 999102}).Delete(&ModelGroupItem{})
	})

	require.NoError(t, repairModelGroupItemInheritance())

	var rows []ModelGroupItem
	require.NoError(t, DB.Where("channel_id IN ?", []int{999101, 999102}).Order("channel_id").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Nil(t, rows[0].Priority) // legacy 0 → inherit
	require.Nil(t, rows[0].Weight)
	require.NotNil(t, rows[1].Priority) // explicit non-zero override kept
	require.Equal(t, int64(9), *rows[1].Priority)

	// Second run is guarded by the option flag: an explicit-0 override set
	// after the fix must survive.
	require.NoError(t, DB.Model(&ModelGroupItem{}).Where("channel_id = ?", 999101).
		Update("priority", pZero).Error)
	require.NoError(t, repairModelGroupItemInheritance())
	var again ModelGroupItem
	require.NoError(t, DB.Where("channel_id = ?", 999101).First(&again).Error)
	require.NotNil(t, again.Priority)
	require.Equal(t, int64(0), *again.Priority)
}
