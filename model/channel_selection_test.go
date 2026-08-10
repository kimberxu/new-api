package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertChannelSelectionTestData(t *testing.T, channels []struct {
	id       int
	priority int64
	weight   uint
}) {
	t.Helper()
	// Truncate leftover rows from other tests sharing the in-memory DB.
	for _, table := range []string{"abilities", "channels"} {
		require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
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
	ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, "", []int{301})
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
	ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, "", []int{401, 402})
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
	ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, "", []int{501, 502})
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
	ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, "", []int{601, 602})
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
	for i := 0; i < 10; i++ {
		ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, "", nil)
		require.NoError(t, err)
		require.NotNil(t, ch)
		assert.Equal(t, int64(4), *ch.Priority, "iteration %d: should always return highest priority with no exclusions", i)
		assert.Equal(t, 702, ch.Id)
	}
}