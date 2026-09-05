package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func insertChannelSelectionTestData(t *testing.T, channels []struct {
	id       int
	priority int64
	weight   uint
}) {
	t.Helper()
	// Truncate leftover rows from other tests sharing the in-memory DB.
	for _, table := range []string{"abilities", "channels", "model_group_items", "model_groups"} {
		require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
	}
	// 组名即路由模型名：default 组 + test-model 只有一个模型组，成员按渠道展开
	var group ModelGroup
	err := DB.Where("name = ?", "test-model").First(&group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		group = ModelGroup{Name: "test-model", Source: GroupSourceManual, Enabled: true}
		require.NoError(t, DB.Create(&group).Error)
	} else {
		require.NoError(t, err)
	}
	for _, ch := range channels {
		require.NoError(t, DB.Create(&Channel{
			Id:       ch.id,
			Type:     constant.ChannelTypeOpenAI,
			Key:      "key",
			Status:   common.ChannelStatusEnabled,
			Name:     "test-channel",
			Weight:   &ch.weight,
			Models:   "test-model",
			Group:    "default",
			Priority: &ch.priority,
		}).Error)
		require.NoError(t, DB.Create(&Ability{
			Group:     "default",
			Model:     "test-model",
			ChannelId: ch.id,
			Enabled:   true,
			Priority:  &ch.priority,
			Weight:    ch.weight,
		}).Error)
		require.NoError(t, DB.Create(&ModelGroupItem{
			GroupId:   group.Id,
			ChannelId: ch.id,
			Model:     "test-model",
			Enabled:   true,
			Priority:  &ch.priority,
			Weight:    &ch.weight,
		}).Error)
	}
}

func TestGetRandomSatisfiedChannelSamePriorityTierReRoll(t *testing.T) {
	// Two channels at priority 3, one at priority 2.
	// Excluding one p3 channel should still return the other p3 channel.
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	insertChannelSelectionTestData(t, []struct {
		id       int
		priority int64
		weight   uint
	}{
		{id: 301, priority: 3, weight: 100},
		{id: 302, priority: 3, weight: 100},
		{id: 303, priority: 2, weight: 100},
	})
	InitChannelCache()

	// Exclude channel 301 -> should still get p3 (302)
	ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, nil, []int{301})
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, 302, ch.Id, "should return the remaining p3 channel, not cascade to p2")
	assert.Equal(t, int64(3), *ch.Priority)
}

func TestGetRandomSatisfiedChannelPriorityCascadeOnTierExhausted(t *testing.T) {
	// Two channels at priority 3, one at priority 2.
	// Excluding all p3 channels should cascade to p2.
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	insertChannelSelectionTestData(t, []struct {
		id       int
		priority int64
		weight   uint
	}{
		{id: 401, priority: 3, weight: 100},
		{id: 402, priority: 3, weight: 100},
		{id: 403, priority: 2, weight: 100},
		{id: 404, priority: 2, weight: 100},
	})
	InitChannelCache()

	// Exclude all p3 channels -> should cascade to p2
	ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, nil, []int{401, 402})
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, int64(2), *ch.Priority, "should cascade to priority 2 when all p3 channels are excluded")
	assert.Contains(t, []int{403, 404}, ch.Id)
}

func TestGetRandomSatisfiedChannelAllExcludedReturnsNil(t *testing.T) {
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	insertChannelSelectionTestData(t, []struct {
		id       int
		priority int64
		weight   uint
	}{
		{id: 501, priority: 3, weight: 100},
		{id: 502, priority: 2, weight: 100},
	})
	InitChannelCache()

	// Exclude all channels -> should return nil, nil
	ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, nil, []int{501, 502})
	require.NoError(t, err)
	assert.Nil(t, ch, "should return nil when all channels are excluded")
}

func TestGetRandomSatisfiedChannelCascadeAcrossThreeTiers(t *testing.T) {
	// Three priority tiers: p3, p2, p1. Exclude p3 and p2 -> should cascade to p1.
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	insertChannelSelectionTestData(t, []struct {
		id       int
		priority int64
		weight   uint
	}{
		{id: 601, priority: 3, weight: 100},
		{id: 602, priority: 2, weight: 100},
		{id: 603, priority: 1, weight: 100},
	})
	InitChannelCache()

	// Exclude p3 and p2
	ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, nil, []int{601, 602})
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, 603, ch.Id, "should cascade to priority 1")
	assert.Equal(t, int64(1), *ch.Priority)
}

func TestGetRandomSatisfiedChannelNoExcludeReturnsHighestPriority(t *testing.T) {
	// Without any exclusions, should always return the highest priority channel.
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	insertChannelSelectionTestData(t, []struct {
		id       int
		priority int64
		weight   uint
	}{
		{id: 701, priority: 1, weight: 100},
		{id: 702, priority: 4, weight: 100},
		{id: 703, priority: 2, weight: 100},
		{id: 704, priority: 3, weight: 100},
	})
	InitChannelCache()

	// Multiple calls should all return a priority 4 channel
	for i := range 10 {
		ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, ch)
		assert.Equal(t, int64(4), *ch.Priority, "iteration %d: should always return highest priority with no exclusions", i)
		assert.Equal(t, 702, ch.Id)
	}
}

func TestGetRandomSatisfiedChannelMultiMemberPerChannelAggregatesSingleWeight(t *testing.T) {
	// Same channel hosts two members: m-a pri8/w1 is the best row (max
	// priority → max weight → min model). The second member m-b pri7/w100
	// must not cause the channel weight to be double-counted, and
	// Resolve must return the best member's upstream (m-a).
	orig := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = orig })

	for _, table := range []string{"channel_disabled_models", "abilities", "channels", "model_group_items", "model_groups"} {
		require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
	}
	pri8 := int64(8)
	pri7 := int64(7)
	w1 := uint(1)
	w100 := uint(100)
	require.NoError(t, DB.Create(&Channel{Id: 9301, Type: constant.ChannelTypeOpenAI, Key: "k", Status: common.ChannelStatusEnabled, Name: "mm-8", Models: "m-a,m-b", Group: "default", Priority: &pri8, Weight: &w1}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 9302, Type: constant.ChannelTypeOpenAI, Key: "k", Status: common.ChannelStatusEnabled, Name: "mm-7", Models: "m-c", Group: "default", Priority: &pri7, Weight: &w100}).Error)
	group := ModelGroup{Name: "test-model", Source: GroupSourceManual, Enabled: true}
	require.NoError(t, DB.Create(&group).Error)
	require.NoError(t, DB.Create(&ModelGroupItem{GroupId: group.Id, ChannelId: 9301, Model: "m-a", Enabled: true, Priority: &pri8, Weight: &w1}).Error)
	require.NoError(t, DB.Create(&ModelGroupItem{GroupId: group.Id, ChannelId: 9301, Model: "m-b", Enabled: true, Priority: &pri7, Weight: &w100}).Error)
	require.NoError(t, DB.Create(&ModelGroupItem{GroupId: group.Id, ChannelId: 9302, Model: "m-c", Enabled: true, Priority: &pri7, Weight: &w100}).Error)
	InitChannelCache()
	t.Cleanup(func() {
		DB.Exec("DELETE FROM channel_disabled_models WHERE channel_id IN (9301,9302)")
		DB.Exec("DELETE FROM model_group_items WHERE channel_id IN (9301,9302)")
		DB.Exec("DELETE FROM model_groups WHERE name = ?", "test-model")
		DB.Exec("DELETE FROM channels WHERE id IN (9301,9302)")
	})

	for i := range 20 {
		ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, ch)
		assert.Equal(t, 9301, ch.Id, "iter %d: tier-8 exclusive, single weight must not cascade to pri7", i)
	}
	assert.Equal(t, "m-a", ResolveModelGroupUpstreamModel("test-model", 9301))
}