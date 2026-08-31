package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type groupMutationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// renameGroupViaHandler invokes UpdateModelGroup directly, mirroring
// PATCH /api/model-groups/:id/name. ApiError always answers HTTP 200, so the
// business success flag is what callers must check.
func renameGroupViaHandler(t *testing.T, id int, name string) groupMutationResponse {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/api/model-groups/%d/name", id),
		strings.NewReader(fmt.Sprintf(`{"name":%q}`, name)),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", id)}}

	UpdateModelGroup(ctx)
	var res groupMutationResponse
	require.NoError(t, common.Unmarshal([]byte(recorder.Body.String()), &res))
	return res
}

func createGroupFixture(t *testing.T, name string, source string) model.ModelGroup {
	t.Helper()

	group := model.ModelGroup{Name: name, Source: source, Enabled: true}
	require.NoError(t, model.DB.Create(&group).Error)
	t.Cleanup(func() { model.DB.Delete(&model.ModelGroup{}, group.Id) })
	return group
}

func TestUpdateModelGroupRenamesManualGroup(t *testing.T) {
	setupModelListControllerTestDB(t)
	group := createGroupFixture(t, "zz-old-name", model.GroupSourceManual)
	require.NoError(t, model.DB.Create(&model.ModelGroupItem{
		GroupId: group.Id, ChannelId: 1, Model: "zz-old-name", Enabled: true,
	}).Error)

	res := renameGroupViaHandler(t, group.Id, "zz-new-name")
	require.True(t, res.Success, res.Message)

	renamed, err := model.GetModelGroupByName("zz-new-name")
	require.NoError(t, err)
	require.NotNil(t, renamed)
	assert.Equal(t, group.Id, renamed.Id, "group identity must be preserved")
	assert.Equal(t, model.GroupSourceManual, renamed.Source)

	old, err := model.GetModelGroupByName("zz-old-name")
	require.NoError(t, err)
	assert.Nil(t, old, "old routable name must no longer resolve")
}

func TestUpdateModelGroupRejectsAutoGroup(t *testing.T) {
	setupModelListControllerTestDB(t)
	group := createGroupFixture(t, "zz-auto-group", model.GroupSourceAuto)

	res := renameGroupViaHandler(t, group.Id, "zz-renamed-auto")
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "system-managed")

	kept, err := model.GetModelGroupById(group.Id)
	require.NoError(t, err)
	require.NotNil(t, kept)
	assert.Equal(t, "zz-auto-group", kept.Name)
}

func TestUpdateModelGroupRejectsDuplicateName(t *testing.T) {
	setupModelListControllerTestDB(t)
	first := createGroupFixture(t, "zz-group-a", model.GroupSourceManual)
	createGroupFixture(t, "zz-group-b", model.GroupSourceManual)

	res := renameGroupViaHandler(t, first.Id, "zz-group-b")
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "already exists")

	kept, err := model.GetModelGroupById(first.Id)
	require.NoError(t, err)
	require.NotNil(t, kept)
	assert.Equal(t, "zz-group-a", kept.Name)
}

func TestUpdateModelGroupRejectsInvalidNames(t *testing.T) {
	setupModelListControllerTestDB(t)
	group := createGroupFixture(t, "zz-group-valid", model.GroupSourceManual)

	for _, name := range []string{"", "  ", "a b", "a,b", "a;b"} {
		res := renameGroupViaHandler(t, group.Id, name)
		assert.False(t, res.Success, "name %q must be rejected", name)
	}

	kept, err := model.GetModelGroupById(group.Id)
	require.NoError(t, err)
	require.NotNil(t, kept)
	assert.Equal(t, "zz-group-valid", kept.Name)
}

func TestUpdateModelGroupAllowsSameName(t *testing.T) {
	setupModelListControllerTestDB(t)
	group := createGroupFixture(t, "zz-same-name", model.GroupSourceManual)

	res := renameGroupViaHandler(t, group.Id, "zz-same-name")
	require.True(t, res.Success, res.Message)

	kept, err := model.GetModelGroupById(group.Id)
	require.NoError(t, err)
	require.NotNil(t, kept)
	assert.Equal(t, "zz-same-name", kept.Name)
}
