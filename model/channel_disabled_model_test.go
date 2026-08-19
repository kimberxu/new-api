package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertChannelDisabledModelTestData(t *testing.T, id int, priority int64) {
	t.Helper()
	for _, table := range []string{"channel_disabled_models", "abilities", "channels"} {
		require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
	}
	require.NoError(t, DB.Create(&Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      "key",
		Status:   common.ChannelStatusEnabled,
		Name:     "test-channel",
		Weight:   &[]uint{100}[0],
		Models:   "test-model",
		Group:    "default",
		Priority: &priority,
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "test-model",
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    100,
	}).Error)
}

func TestChannelDisabledModel_DBPathExcludesModel(t *testing.T) {
	insertChannelDisabledModelTestData(t, 501, 10)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	// Without a disable record the channel is selectable.
	ch, err := GetChannel("default", "test-model", 0, "")
	require.NoError(t, err)
	require.NotNil(t, ch, "channel should be selectable before model-level disable")
	assert.Equal(t, 501, ch.Id)

	// Disable the model -> DB path excludes the (channel, model) pair.
	require.NoError(t, AddChannelDisabledModels(501, []string{"test-model"}, "auto", "model test failure"))
	ch, err = GetChannel("default", "test-model", 0, "")
	require.NoError(t, err)
	assert.Nil(t, ch, "DB path must exclude a model-level disabled channel")

	// Re-enable -> selectable again.
	require.NoError(t, EnableChannelModelDisabled(501, "test-model", ""))
	ch, err = GetChannel("default", "test-model", 0, "")
	require.NoError(t, err)
	require.NotNil(t, ch, "channel should be selectable after re-enabling the model")
	assert.Equal(t, 501, ch.Id)
}

func TestChannelDisabledModel_CachePathExcludesModel(t *testing.T) {
	insertChannelDisabledModelTestData(t, 502, 10)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	require.NoError(t, AddChannelDisabledModels(502, []string{"test-model"}, "manual", "admin disabled"))
	InitChannelCache()

	// The disabled (channel, model) pair must not be in the routing index.
	ch, err := GetRandomSatisfiedChannel("default", "test-model", 0, "", nil)
	require.NoError(t, err)
	assert.Nil(t, ch, "cache path must exclude a model-level disabled channel")

	// Re-enable -> back in the index.
	require.NoError(t, EnableChannelModelDisabled(502, "test-model", ""))
	InitChannelCache()
	ch, err = GetRandomSatisfiedChannel("default", "test-model", 0, "", nil)
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, 502, ch.Id)
}

func TestChannelDisabledModel_AddIdempotentAndCleanup(t *testing.T) {
	insertChannelDisabledModelTestData(t, 503, 10)

	// First add inserts with source=auto.
	require.NoError(t, AddChannelDisabledModels(503, []string{"test-model"}, "auto", "first"))
	records, err := GetChannelDisabledModels(503)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "auto", records[0].Source)
	assert.Equal(t, "first", records[0].Reason)

	// Re-adding the same model with a different source is ignored (OnConflict
	// DoNothing) — original source/reason preserved.
	require.NoError(t, AddChannelDisabledModels(503, []string{"test-model"}, "manual", "second"))
	records, err = GetChannelDisabledModels(503)
	require.NoError(t, err)
	require.Len(t, records, 1, "duplicate add must not create another row")
	assert.Equal(t, "auto", records[0].Source)
	assert.Equal(t, "first", records[0].Reason)

	// Empty source defaults to "manual".
	require.NoError(t, AddChannelDisabledModels(503, []string{"other-model"}, "", "no source"))
	records, err = GetChannelDisabledModels(503)
	require.NoError(t, err)
	assert.Len(t, records, 2)
	found := false
	for _, r := range records {
		if r.Model == "other-model" {
			found = true
			assert.Equal(t, "manual", r.Source)
		}
	}
	assert.True(t, found, "source should default to manual when empty")

	// Channel-scoped cleanup removes everything.
	require.NoError(t, DeleteChannelDisabledModelsByChannelId(503))
	records, err = GetChannelDisabledModels(503)
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestChannelDisabledModel_EnableBySource(t *testing.T) {
	insertChannelDisabledModelTestData(t, 504, 10)

	require.NoError(t, AddChannelDisabledModels(504, []string{"test-model"}, "auto", "auto reason"))
	require.NoError(t, AddChannelDisabledModels(504, []string{"test-model-2"}, "manual", "manual reason"))

	// Enabling with source="auto" only clears the auto record; the manual
	// record for test-model-2 must survive.
	require.NoError(t, EnableChannelModelDisabled(504, "test-model", "auto"))
	records, err := GetChannelDisabledModels(504)
	require.NoError(t, err)
	require.Len(t, records, 1, "auto record cleared, manual record remains")
	assert.Equal(t, "test-model-2", records[0].Model)
	assert.Equal(t, "manual", records[0].Source)

	// Enabling a manual record with source="auto" must NOT clear it.
	require.NoError(t, EnableChannelModelDisabled(504, "test-model-2", "auto"))
	records, err = GetChannelDisabledModels(504)
	require.NoError(t, err)
	require.Len(t, records, 1, "manual record must not be cleared by auto-source enable")
	assert.Equal(t, "manual", records[0].Source)

	// Empty source clears any source.
	require.NoError(t, EnableChannelModelDisabled(504, "test-model-2", ""))
	records, err = GetChannelDisabledModels(504)
	require.NoError(t, err)
	assert.Empty(t, records)
}