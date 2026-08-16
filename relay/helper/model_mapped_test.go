package helper

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPickWeightedModel_AllZeroWeight(t *testing.T) {
	items := []relaycommon.WeightedModelItem{
		{Model: "a", Weight: 0},
		{Model: "b", Weight: 0},
		{Model: "c", Weight: 0},
	}
	seen := make(map[string]int)
	for i := 0; i < 900; i++ {
		seen[pickWeightedModel(items)]++
	}
	// 三个模型都应该被选中（等概率）
	assert.GreaterOrEqual(t, len(seen), 2, "should pick at least 2 different models")
}

func TestPickWeightedModel_SingleItem(t *testing.T) {
	items := []relaycommon.WeightedModelItem{
		{Model: "only-one", Weight: 10},
	}
	for i := 0; i < 10; i++ {
		assert.Equal(t, "only-one", pickWeightedModel(items))
	}
}

func TestPickWeightedModel_WeightDistribution(t *testing.T) {
	items := []relaycommon.WeightedModelItem{
		{Model: "heavy", Weight: 90},
		{Model: "light", Weight: 10},
	}
	heavyCount := 0
	const n = 1000
	for i := 0; i < n; i++ {
		if pickWeightedModel(items) == "heavy" {
			heavyCount++
		}
	}
	// 期望 ~90%，允许上下 15% 的浮动（统计误差）
	assert.InDelta(t, 900, heavyCount, 150, "heavy should be picked ~90%% of the time")
}

func TestResolveModelMappingValue_String(t *testing.T) {
	result, err := resolveModelMappingValue("deepseek-v4-flash")
	require.NoError(t, err)
	assert.Equal(t, "deepseek-v4-flash", result)
}

func TestResolveModelMappingValue_StringWithSpaces(t *testing.T) {
	result, err := resolveModelMappingValue("  deepseek-v4-flash  ")
	require.NoError(t, err)
	assert.Equal(t, "deepseek-v4-flash", result)
}

func TestResolveModelMappingValue_EmptyString(t *testing.T) {
	result, err := resolveModelMappingValue("")
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestResolveModelMappingValue_WeightedArray(t *testing.T) {
	raw := []any{
		map[string]any{"model": "deepseek-v4-flash", "weight": float64(5)},
		map[string]any{"model": "deepseek-ai/deepseek-v4-flash-0731", "weight": float64(3)},
	}
	result, err := resolveModelMappingValue(raw)
	require.NoError(t, err)
	assert.Contains(t, []string{"deepseek-v4-flash", "deepseek-ai/deepseek-v4-flash-0731"}, result)
}

func TestResolveModelMappingValue_WeightedArrayZeroWeight(t *testing.T) {
	raw := []any{
		map[string]any{"model": "m1", "weight": float64(0)},
		map[string]any{"model": "m2", "weight": float64(0)},
	}
	result, err := resolveModelMappingValue(raw)
	require.NoError(t, err)
	assert.Contains(t, []string{"m1", "m2"}, result)
}

func TestResolveModelMappingValue_WeightMissingDefaultsToOne(t *testing.T) {
	// 单条目，weight 字段缺失 → 默认 1，必被选中
	raw := []any{
		map[string]any{"model": "only-one"},
	}
	result, err := resolveModelMappingValue(raw)
	require.NoError(t, err)
	assert.Equal(t, "only-one", result)
}

func TestResolveModelMappingValue_WeightNullDefaultsToOne(t *testing.T) {
	// 单条目，weight 为 null → 默认 1，必被选中
	raw := []any{
		map[string]any{"model": "only-one", "weight": nil},
	}
	result, err := resolveModelMappingValue(raw)
	require.NoError(t, err)
	assert.Equal(t, "only-one", result)
}

func TestResolveModelMappingValue_WeightMissingWithOtherWeights(t *testing.T) {
	// 混合：缺失 weight 的条目与显式 weight 并存，缺失者仍可被选中（概率 ≥ 1/(5+1+1)）
	raw := []any{
		map[string]any{"model": "m1", "weight": float64(5)},
		map[string]any{"model": "m2"},
		map[string]any{"model": "m3", "weight": nil},
	}
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		result, err := resolveModelMappingValue(raw)
		require.NoError(t, err)
		seen[result] = true
	}
	assert.True(t, seen["m1"], "m1 should be selectable")
	assert.True(t, seen["m2"], "m2 (weight missing) should be selectable with default weight 1")
	assert.True(t, seen["m3"], "m3 (weight null) should be selectable with default weight 1")
}

func TestResolveModelMappingValue_InvalidWeightType(t *testing.T) {
	raw := []any{
		map[string]any{"model": "m1", "weight": "heavy"},
	}
	_, err := resolveModelMappingValue(raw)
	require.Error(t, err)
}

func TestResolveModelMappingValue_InvalidItem(t *testing.T) {
	raw := []any{"not-a-map"}
	_, err := resolveModelMappingValue(raw)
	require.Error(t, err)
}

func TestResolveModelMappingValue_InvalidModelEmpty(t *testing.T) {
	raw := []any{
		map[string]any{"model": "", "weight": float64(5)},
	}
	_, err := resolveModelMappingValue(raw)
	require.Error(t, err)
}

func TestResolveModelMappingValue_InvalidWeightNegative(t *testing.T) {
	raw := []any{
		map[string]any{"model": "m1", "weight": float64(-1)},
	}
	_, err := resolveModelMappingValue(raw)
	require.Error(t, err)
}

func TestResolveModelMappingValue_UnsupportedType(t *testing.T) {
	_, err := resolveModelMappingValue(float64(42))
	require.Error(t, err)
	_, err = resolveModelMappingValue(true)
	require.Error(t, err)
	_, err = resolveModelMappingValue(nil)
	require.Error(t, err)
}

// --- ModelMappedHelper 集成测试 ---

func setupModelMappingTest(mappingJSON string) (*gin.Context, *relaycommon.RelayInfo) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if mappingJSON != "" {
		c.Set("model_mapping", mappingJSON)
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "ds-v4",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	return c, info
}

func TestModelMappedHelper_NoMapping(t *testing.T) {
	c, info := setupModelMappingTest("")
	info.UpstreamModelName = "ds-v4"
	err := ModelMappedHelper(c, info, nil)
	require.NoError(t, err)
	assert.False(t, info.IsModelMapped)
	assert.Equal(t, "ds-v4", info.UpstreamModelName)
}

func TestModelMappedHelper_EmptyMapping(t *testing.T) {
	c, info := setupModelMappingTest("{}")
	info.UpstreamModelName = "ds-v4"
	err := ModelMappedHelper(c, info, nil)
	require.NoError(t, err)
	assert.False(t, info.IsModelMapped)
	assert.Equal(t, "ds-v4", info.UpstreamModelName)
}

func TestModelMappedHelper_OneToOne(t *testing.T) {
	c, info := setupModelMappingTest(`{"ds-v4": "deepseek-v4-flash"}`)
	info.UpstreamModelName = "ds-v4"
	err := ModelMappedHelper(c, info, nil)
	require.NoError(t, err)
	assert.True(t, info.IsModelMapped)
	assert.Equal(t, "deepseek-v4-flash", info.UpstreamModelName)
}

func TestModelMappedHelper_OneToMany(t *testing.T) {
	c, info := setupModelMappingTest(`{
		"ds-v4": [
			{"model": "deepseek-v4-flash", "weight": 5},
			{"model": "deepseek-ai/deepseek-v4-flash-0731", "weight": 3},
			{"model": "deepseek-ai/deepseek-v4-flash", "weight": 2}
		]
	}`)
	info.UpstreamModelName = "ds-v4"
	err := ModelMappedHelper(c, info, nil)
	require.NoError(t, err)
	assert.True(t, info.IsModelMapped)
	assert.Contains(t, []string{
		"deepseek-v4-flash",
		"deepseek-ai/deepseek-v4-flash-0731",
		"deepseek-ai/deepseek-v4-flash",
	}, info.UpstreamModelName)
}

func TestModelMappedHelper_ChainMapping(t *testing.T) {
	c, info := setupModelMappingTest(`{
		"ds-v4": "ds-v4-alias",
		"ds-v4-alias": "deepseek-v4-flash"
	}`)
	info.UpstreamModelName = "ds-v4"
	err := ModelMappedHelper(c, info, nil)
	require.NoError(t, err)
	assert.True(t, info.IsModelMapped)
	assert.Equal(t, "deepseek-v4-flash", info.UpstreamModelName)
}

func TestModelMappedHelper_WeightedThenChain(t *testing.T) {
	// 加权选中后继续链式映射
	c, info := setupModelMappingTest(`{
		"ds-v4": [{"model": "ds-v4-alias", "weight": 1}],
		"ds-v4-alias": "deepseek-v4-flash"
	}`)
	info.UpstreamModelName = "ds-v4"
	err := ModelMappedHelper(c, info, nil)
	require.NoError(t, err)
	assert.True(t, info.IsModelMapped)
	assert.Equal(t, "deepseek-v4-flash", info.UpstreamModelName)
}

func TestModelMappedHelper_SelfMapping(t *testing.T) {
	c, info := setupModelMappingTest(`{"ds-v4": "ds-v4"}`)
	info.UpstreamModelName = "ds-v4"
	err := ModelMappedHelper(c, info, nil)
	require.NoError(t, err)
	assert.False(t, info.IsModelMapped)
	assert.Equal(t, "ds-v4", info.UpstreamModelName)
}

func TestModelMappedHelper_Cycle(t *testing.T) {
	c, info := setupModelMappingTest(`{
		"ds-v4": "alias-a",
		"alias-a": "alias-b",
		"alias-b": "alias-a"
	}`)
	info.UpstreamModelName = "ds-v4"
	err := ModelMappedHelper(c, info, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model_mapping_contains_cycle")
}

func TestModelMappedHelper_RequestSetModel(t *testing.T) {
	c, info := setupModelMappingTest(`{"ds-v4": "deepseek-v4-flash"}`)
	info.UpstreamModelName = "ds-v4"
	req := &dto.GeneralOpenAIRequest{Model: "ds-v4"}
	err := ModelMappedHelper(c, info, req)
	require.NoError(t, err)
	assert.Equal(t, "deepseek-v4-flash", req.Model)
}

func TestModelMappedHelper_KeyNotInMapping(t *testing.T) {
	c, info := setupModelMappingTest(`{"gpt-4": "gpt-4-turbo"}`)
	info.OriginModelName = "ds-v4"
	info.UpstreamModelName = "ds-v4"
	err := ModelMappedHelper(c, info, nil)
	require.NoError(t, err)
	assert.False(t, info.IsModelMapped)
	assert.Equal(t, "ds-v4", info.UpstreamModelName)
}

func TestResolveModelMappingValue_WeightedArrayEmpty(t *testing.T) {
	raw := []any{}
	_, err := resolveModelMappingValue(raw)
	require.Error(t, err)
}