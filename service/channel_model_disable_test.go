package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// migrateChannelDisabledModels ensures the channel_disabled_models table
// exists (idempotent; the package TestMain AutoMigrate list does not include
// it) and registers cleanup so tests stay isolated on the shared in-memory DB.
func migrateChannelDisabledModels(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.ChannelDisabledModel{}))
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM channel_disabled_models")
	})
}

func TestInitialBanStageForStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       int
	}{
		{"401 starts at 16h stage", 401, 5},
		{"200 starts at base stage", 200, 0},
		{"403 starts at base stage", 403, 0},
		{"500 starts at base stage", 500, 0},
		{"404 starts at base stage", 404, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, initialBanStageForStatusCode(tt.statusCode))
		})
	}
}

func TestDisableChannelModel_FirstBan(t *testing.T) {
	migrateChannelDisabledModels(t)

	require.NoError(t, DisableChannelModel(1, "gpt-4", "upstream boom", 200))

	record, err := model.GetChannelDisabledModel(1, "gpt-4")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, 0, record.BanStage)
	assert.Equal(t, "auto", record.Source)
	assert.Equal(t, "upstream boom", record.Reason)
	assert.WithinDuration(t, time.Now().Add(30*time.Minute), time.Unix(record.BannedUntil, 0), time.Minute)
}

func TestDisableChannelModel_401StartAt16h(t *testing.T) {
	migrateChannelDisabledModels(t)

	require.NoError(t, DisableChannelModel(1, "gpt-4", "invalid key", 401))

	record, err := model.GetChannelDisabledModel(1, "gpt-4")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, 5, record.BanStage)
	assert.WithinDuration(t, time.Now().Add(16*time.Hour), time.Unix(record.BannedUntil, 0), time.Minute)
}

func TestExtendChannelModelBan_StageAdvance(t *testing.T) {
	migrateChannelDisabledModels(t)

	require.NoError(t, DisableChannelModel(2, "gpt-4", "boom", 200))
	require.NoError(t, ExtendChannelModelBan(2, "gpt-4"))

	record, err := model.GetChannelDisabledModel(2, "gpt-4")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, 1, record.BanStage)
	assert.WithinDuration(t, time.Now().Add(1*time.Hour), time.Unix(record.BannedUntil, 0), time.Minute)

	require.NoError(t, ExtendChannelModelBan(2, "gpt-4"))
	record, err = model.GetChannelDisabledModel(2, "gpt-4")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, 2, record.BanStage)
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), time.Unix(record.BannedUntil, 0), time.Minute)
}

func TestExtendChannelModelBan_401EscalatesToPermanent(t *testing.T) {
	migrateChannelDisabledModels(t)

	require.NoError(t, DisableChannelModel(3, "gpt-4", "invalid key", 401))

	// 401 starts at stage 5 (16h); two failed recovery probes reach
	// permanent (stage 7, BannedUntil=0).
	require.NoError(t, ExtendChannelModelBan(3, "gpt-4"))
	record, err := model.GetChannelDisabledModel(3, "gpt-4")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, 6, record.BanStage)
	assert.WithinDuration(t, time.Now().Add(32*time.Hour), time.Unix(record.BannedUntil, 0), time.Minute)

	require.NoError(t, ExtendChannelModelBan(3, "gpt-4"))
	record, err = model.GetChannelDisabledModel(3, "gpt-4")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, 7, record.BanStage)
	assert.Equal(t, int64(0), record.BannedUntil, "stage 7 must be permanent (BannedUntil=0)")
}

func TestExtendChannelModelBan_RecordGoneNoop(t *testing.T) {
	migrateChannelDisabledModels(t)

	require.NoError(t, ExtendChannelModelBan(4, "never-banned-model"))

	record, err := model.GetChannelDisabledModel(4, "never-banned-model")
	require.NoError(t, err)
	assert.Nil(t, record)
}

func TestDisableChannelModel_RepeatedDisableRefreshesStage(t *testing.T) {
	migrateChannelDisabledModels(t)

	require.NoError(t, DisableChannelModel(5, "gpt-4", "boom", 200))
	// A later failure with a different status code resets the ban to that
	// status code's initial stage (no escalation from repeat failures —
	// escalation only happens via failed recovery probes).
	require.NoError(t, DisableChannelModel(5, "gpt-4", "invalid key", 401))

	record, err := model.GetChannelDisabledModel(5, "gpt-4")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, 5, record.BanStage)
	assert.WithinDuration(t, time.Now().Add(16*time.Hour), time.Unix(record.BannedUntil, 0), time.Minute)
}
