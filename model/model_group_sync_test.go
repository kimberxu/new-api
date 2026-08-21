package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression: rebuild must drop auto groups whose members all disappeared,
// while keeping non-empty auto groups and never touching manual groups.
func TestDeleteEmptyAutoModelGroups(t *testing.T) {
	emptyAuto := ModelGroup{Name: "zz-empty-auto", Source: GroupSourceAuto, Enabled: true}
	require.NoError(t, DB.Create(&emptyAuto).Error)
	fullAuto := ModelGroup{Name: "zz-full-auto", Source: GroupSourceAuto, Enabled: true}
	require.NoError(t, DB.Create(&fullAuto).Error)
	emptyManual := ModelGroup{Name: "zz-empty-manual", Source: GroupSourceManual, Enabled: true}
	require.NoError(t, DB.Create(&emptyManual).Error)
	t.Cleanup(func() {
		DB.Where("name IN ?", []string{emptyAuto.Name, fullAuto.Name, emptyManual.Name}).Delete(&ModelGroup{})
		DB.Where("group_id = ? OR ref_group_id = ?", fullAuto.Id, emptyAuto.Id).Delete(&ModelGroupReference{})
	})

	require.NoError(t, DB.Create(&ModelGroupItem{
		GroupId: fullAuto.Id, ChannelId: 999501, Model: "zz-full-auto", Enabled: true,
	}).Error)
	t.Cleanup(func() { DB.Where("channel_id = ?", 999501).Delete(&ModelGroupItem{}) })

	// A reference pointing at the doomed empty auto group must be cleaned up.
	require.NoError(t, DB.Create(&ModelGroupReference{GroupId: fullAuto.Id, RefGroupId: emptyAuto.Id}).Error)

	removed, err := DeleteEmptyAutoModelGroups()
	require.NoError(t, err)
	assert.Equal(t, []string{"zz-empty-auto"}, removed)

	gone, err := GetModelGroupById(emptyAuto.Id)
	require.NoError(t, err)
	assert.Nil(t, gone, "empty auto group must be deleted")

	kept, err := GetModelGroupById(fullAuto.Id)
	require.NoError(t, err, "non-empty auto group must survive")
	require.NotNil(t, kept)

	keptManual, err := GetModelGroupById(emptyManual.Id)
	require.NoError(t, err, "manual groups are never touched")
	require.NotNil(t, keptManual)

	var refs []ModelGroupReference
	require.NoError(t, DB.Where("ref_group_id = ?", emptyAuto.Id).Find(&refs).Error)
	assert.Empty(t, refs, "references to the deleted group must be removed")

	require.NoError(t, DB.Delete(&ModelGroup{}, kept.Id).Error)
}
