package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
)

// resolveModelMappingValue 解析 model_mapping 中单个 value，返回最终的具体模型名。
//   - string: 直接返回（trimmed），沿用现有 1:1 逻辑
//   - []any: 按权重随机选一个 WeightedModelItem.model
//   其他类型或空字符串则返回 ("", nil)
func resolveModelMappingValue(rawValue any) (string, error) {
	switch v := rawValue.(type) {
	case string:
		return strings.TrimSpace(v), nil
	case []any:
		items := make([]relaycommon.WeightedModelItem, 0, len(v))
		for _, item := range v {
			itemMap, ok := item.(map[string]any)
			if !ok {
				return "", errors.New("invalid_model_mapping_weighted_item")
			}
			modelRaw, _ := itemMap["model"].(string)
			model := strings.TrimSpace(modelRaw)
			if model == "" {
				return "", errors.New("invalid_model_mapping_weighted_item")
			}
			// weight 留空（缺失或 null）时默认 1
			weight := 1
			if weightRaw, exists := itemMap["weight"]; exists && weightRaw != nil {
				weightFloat, ok := weightRaw.(float64)
				if !ok {
					return "", errors.New("invalid_model_mapping_weight")
				}
				weight = int(weightFloat)
			}
			if weight < 0 {
				return "", fmt.Errorf("invalid_model_mapping_weight: %d", weight)
			}
			items = append(items, relaycommon.WeightedModelItem{
				Model:  model,
				Weight: weight,
			})
		}
		if len(items) == 0 {
			return "", errors.New("invalid_model_mapping_weighted_item")
		}
		return pickWeightedModel(items), nil
	default:
		// nil, bool, float64, map[string]any 等类型均不支持
		return "", errors.New("invalid_model_mapping_value")
	}
}

// pickWeightedModel 按权重随机选择一个模型。
// 当所有 weight 均为 0 时，等概率随机选择。
func pickWeightedModel(items []relaycommon.WeightedModelItem) string {
	totalWeight := 0
	for _, item := range items {
		totalWeight += item.Weight
	}
	if totalWeight <= 0 {
		// 等概率回退
		return items[rand.Intn(len(items))].Model
	}
	r := rand.Intn(totalWeight)
	for _, item := range items {
		r -= item.Weight
		if r < 0 {
			return item.Model
		}
	}
	return items[len(items)-1].Model
}

// ModelMappedHelper 应用渠道的 model_mapping 将 OriginModelName 映射为 UpstreamModelName。
//
// 支持两种格式：
//   - 1:1 映射：{"ds-v4": "deepseek-v4-flash"}
//   - 1:N 加权映射：{"ds-v4": [{"model": "deepseek-v4-flash", "weight": 5}, ...]}
//
// 支持链式映射：A → B → C，最终使用链尾的模型名。
// 加权映射在链式第一步 resolve 为具体模型名后继续链式解析。
func ModelMappedHelper(c *gin.Context, info *relaycommon.RelayInfo, request dto.Request) error {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &relaycommon.ChannelMeta{}
	}

	modelMapping := c.GetString("model_mapping")
	if modelMapping == "" || modelMapping == "{}" {
		return nil
	}

	modelMap := make(map[string]any)
	if err := json.Unmarshal([]byte(modelMapping), &modelMap); err != nil {
		return fmt.Errorf("unmarshal_model_mapping_failed")
	}

	currentModel := info.OriginModelName
	visitedModels := map[string]bool{
		currentModel: true,
	}
	for {
		rawValue, exists := modelMap[currentModel]
		if !exists || rawValue == nil {
			break
		}
		mappedModel, err := resolveModelMappingValue(rawValue)
		if err != nil {
			return err
		}
		if mappedModel == "" {
			break
		}

		// 循环检测
		if visitedModels[mappedModel] {
			if mappedModel == currentModel {
				// 自映射（A→A）→ 无映射
				if currentModel == info.OriginModelName {
					info.IsModelMapped = false
					return nil
				}
				// 链尾自映射（A→B→B）→ 停在 B
				info.IsModelMapped = true
				break
			}
			return errors.New("model_mapping_contains_cycle")
		}
		visitedModels[mappedModel] = true
		currentModel = mappedModel
		info.IsModelMapped = true
	}

	if info.IsModelMapped {
		info.UpstreamModelName = currentModel
	}

	if request != nil {
		request.SetModelName(info.UpstreamModelName)
	}
	return nil
}