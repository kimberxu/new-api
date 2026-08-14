package common

// WeightedModelItem 表示加权模型映射中的一个条目。
// 当 model_mapping 中某个 value 是数组时，每个元素对应一个 WeightedModelItem。
type WeightedModelItem struct {
	Model  string `json:"model"`
	Weight int    `json:"weight"`
}