package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupUpstreamModelFixture creates a manual group "ox" whose single member is
// the real upstream model "gpt-4o" on channel 8001, plus an auto-style member
// whose model equals its group name ("same-name-model" on channel 8002).
func setupUpstreamModelFixture(t *testing.T) {
	t.Helper()
	for _, table := range []string{"channels", "model_group_items", "model_groups"} {
		require.NoError(t, DB.Exec("DELETE FROM " + table).Error)
	}
	require.NoError(t, DB.Create(&Channel{
		Id:     8001,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "key",
		Status: common.ChannelStatusEnabled,
		Name:   "up-test-1",
		Models: "gpt-4o",
		Group:  "default",
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id:     8002,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "key",
		Status: common.ChannelStatusEnabled,
		Name:   "up-test-2",
		Models: "same-name-model",
		Group:  "default",
	}).Error)

	group := ModelGroup{Name: "ox", Source: GroupSourceManual, Enabled: true}
	require.NoError(t, DB.Create(&group).Error)
	autoGroup := ModelGroup{Name: "same-name-model", Source: GroupSourceAuto, Enabled: true}
	require.NoError(t, DB.Create(&autoGroup).Error)
	require.NoError(t, DB.Create(&ModelGroupItem{
		GroupId: group.Id, ChannelId: 8001, Model: "gpt-4o", Enabled: true,
	}).Error)
	require.NoError(t, DB.Create(&ModelGroupItem{
		GroupId: autoGroup.Id, ChannelId: 8002, Model: "same-name-model", Enabled: true,
	}).Error)

	t.Cleanup(func() {
		for _, table := range []string{"channels", "model_group_items", "model_groups"} {
			DB.Exec("DELETE FROM " + table)
		}
	})
}

func TestApplyModelGroupMemberMappingCachePath(t *testing.T) {
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	setupUpstreamModelFixture(t)
	InitChannelCache()

	// Member's upstream model injected as a synthetic mapping entry.
	got := ApplyModelGroupMemberMapping("", "ox", 8001)
	assert.JSONEq(t, `{"ox":"gpt-4o"}`, got)

	// Existing non-conflicting entries preserved (incl. weighted arrays).
	got = ApplyModelGroupMemberMapping(`{"alias-x":[{"model":"m-x","weight":2}]}`, "ox", 8001)
	assert.JSONEq(t, `{"alias-x":[{"model":"m-x","weight":2}],"ox":"gpt-4o"}`, got)

	// Explicit channel mapping wins.
	got = ApplyModelGroupMemberMapping(`{"ox":"explicit-target"}`, "ox", 8001)
	assert.JSONEq(t, `{"ox":"explicit-target"}`, got)

	// Member model identical to the group name → no rewrite.
	assert.Empty(t, ApplyModelGroupMemberMapping("", "same-name-model", 8002))

	// Unknown channel/group → untouched.
	assert.Equal(t, "", ApplyModelGroupMemberMapping("", "nope", 9999))
}

func TestApplyModelGroupMemberMappingDBFallbackPath(t *testing.T) {
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	setupUpstreamModelFixture(t)

	got := ApplyModelGroupMemberMapping("", "ox", 8001)
	assert.JSONEq(t, `{"ox":"gpt-4o"}`, got)

	// Disabled member must not inject a mapping.
	require.NoError(t, DB.Model(&ModelGroupItem{}).
		Where("channel_id = ?", 8001).Update("enabled", false).Error)
	assert.Equal(t, "", ApplyModelGroupMemberMapping("", "ox", 8001))
}
