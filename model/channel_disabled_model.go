package model

import (
	"errors"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChannelDisabledModel records a model-level disable for one channel.
// It is deliberately separate from the abilities.enabled column: the
// channel.status ↔ ability.enabled sync invariant (UpdateChannelStatus defer)
// must not erase model-level disables when a channel is re-enabled.
type ChannelDisabledModel struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	ChannelId int    `json:"channel_id" gorm:"index;uniqueIndex:idx_channel_model"`
	Model     string `json:"model" gorm:"type:varchar(128);uniqueIndex:idx_channel_model"`
	// Source is "manual" or "auto"; the default is applied in code
	// (AddChannelDisabledModels) rather than via a gorm default tag to avoid
	// cross-database AutoMigrate churn.
	Source string `json:"source" gorm:"type:varchar(16)"`
	Reason string `json:"reason" gorm:"type:text"`
	// LastError stores the most recent upstream error from a recovery
	// probe or manual test, so the model-group UI can show why the
	// ban was extended.
	LastError string `json:"last_error" gorm:"type:text"`
	// [personal] BannedUntil is the unix timestamp after which an auto ban
	// expires (0 = permanent). Only auto-sourced bans carry a deadline; the
	// periodic recovery probe re-tests the model when it passes.
	BannedUntil int64 `json:"banned_until" gorm:"bigint;default:0"`
	// BanStage is the escalation stage of an auto ban: 0..6 map to
	// modelBanDurations, 7 = permanent. Manual bans keep 0 and BannedUntil=0.
	BanStage  int   `json:"ban_stage" gorm:"default:0"`
	CreatedAt int64 `json:"created_at" gorm:"bigint;autoCreateTime"`
}

// AddChannelDisabledModels inserts model-level disable records, ignoring
// duplicates (an existing record's source/reason is never overwritten).
// An empty source defaults to "manual".
func AddChannelDisabledModels(channelId int, models []string, source string, reason string) error {
	if source == "" {
		source = "manual"
	}
	records := make([]ChannelDisabledModel, 0, len(models))
	for _, modelName := range models {
		records = append(records, ChannelDisabledModel{
			ChannelId: channelId,
			Model:     modelName,
			Source:    source,
			Reason:    reason,
		})
	}
	if len(records) == 0 {
		return nil
	}
	for _, chunk := range lo.Chunk(records, 50) {
		err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteChannelDisabledModels removes model-level disable records for the
// given channel and models (any source).
func DeleteChannelDisabledModels(channelId int, models []string) error {
	if len(models) == 0 {
		return nil
	}
	return DB.Where("channel_id = ? AND model IN ?", channelId, models).Delete(&ChannelDisabledModel{}).Error
}

// DeleteChannelDisabledModelsByChannelId removes all model-level disable
// records for one channel.
func DeleteChannelDisabledModelsByChannelId(channelId int) error {
	return DB.Where("channel_id = ?", channelId).Delete(&ChannelDisabledModel{}).Error
}

// DeleteChannelDisabledModelsByChannelIds removes model-level disable records
// for multiple channels (bulk channel deletion cleanup).
func DeleteChannelDisabledModelsByChannelIds(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return DB.Where("channel_id IN ?", ids).Delete(&ChannelDisabledModel{}).Error
}

// GetChannelDisabledModels returns model-level disable records for one channel.
func GetChannelDisabledModels(channelId int) ([]ChannelDisabledModel, error) {
	var records []ChannelDisabledModel
	err := DB.Where("channel_id = ?", channelId).Find(&records).Error
	return records, err
}

// GetChannelDisabledModel reads a single model-level disable record.
// Returns nil, nil when absent (gorm.ErrRecordNotFound swallowed).
func GetChannelDisabledModel(channelId int, model string) (*ChannelDisabledModel, error) {
	var record ChannelDisabledModel
	err := DB.Where("channel_id = ? AND model = ?", channelId, model).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &record, err
}

// GetAllChannelDisabledModels returns every model-level disable record (used
// to build the in-memory channel cache).
func GetAllChannelDisabledModels() ([]ChannelDisabledModel, error) {
	var records []ChannelDisabledModel
	err := DB.Find(&records).Error
	return records, err
}

// EnableChannelModelDisabled removes a model-level disable record. An empty
// source matches any source; otherwise only the given source is cleared.
func EnableChannelModelDisabled(channelId int, model string, source string) error {
	query := DB.Where("channel_id = ? AND model = ?", channelId, model)
	if source != "" {
		query = query.Where("source = ?", source)
	}
	return query.Delete(&ChannelDisabledModel{}).Error
}

// SetChannelDisabledModelBanStage updates the ban stage, deadline and most
// recent probe error of an existing model-level disable record. No-op (nil)
// when the record is absent.
func SetChannelDisabledModelBanStage(channelId int, model string, banStage int, bannedUntil int64, lastError string) error {
	return DB.Model(&ChannelDisabledModel{}).
		Where("channel_id = ? AND model = ?", channelId, model).
		Update("ban_stage", banStage).
		Update("banned_until", bannedUntil).
		Update("last_error", lastError).Error
}

// SetChannelDisabledModelError updates only the last_error field of an existing
// model-level disable record. No-op when the record is absent.
func SetChannelDisabledModelError(channelId int, model string, lastError string) error {
	return DB.Model(&ChannelDisabledModel{}).
		Where("channel_id = ? AND model = ?", channelId, model).
		Update("last_error", lastError).Error
}
