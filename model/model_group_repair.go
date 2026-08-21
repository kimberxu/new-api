package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// [personal] One-time data repair for model-group member inheritance.
//
// ModelGroupItem.Priority/Weight previously carried gorm "default:0" tags.
// GORM omits zero-valued fields (including nil pointers) from INSERT when a
// default is declared and backfills the DB default afterwards, so members
// meant to inherit the channel priority/weight (NULL) were persisted as an
// explicit 0 override — breaking inheritance in routing, the cache overrides
// map, and the UI.
//
// repairModelGroupItemInheritance nulls those 0 values once. The option flag
// guards against re-running on later restarts, so deliberate explicit-0
// overrides made after the fix are never rewritten.
func repairModelGroupItemInheritance() error {
	const flagKey = "ModelGroupInheritRepairDone"
	var opt Option
	err := DB.Where("key = ?", flagKey).First(&opt).Error
	if err == nil {
		return nil // already repaired
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := DB.Exec("UPDATE model_group_items SET priority = NULL WHERE priority = 0").Error; err != nil {
		return err
	}
	if err := DB.Exec("UPDATE model_group_items SET weight = NULL WHERE weight = 0").Error; err != nil {
		return err
	}
	common.SysLog("model group items: reset 0-valued priority/weight to NULL (inherit)")
	return DB.Create(&Option{Key: flagKey, Value: "true"}).Error
}
