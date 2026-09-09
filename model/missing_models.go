package model

// GetMissingModels returns model names that are referenced in the system
func GetMissingModels() ([]string, error) {
	// 1. 获取所有已启用模型（去重）：模型组路由来源 + 渠道 ability 直配来源，
	// 渠道直接配置但未入组的模型同样属于"系统引用了但缺 metadata"的范畴。
	modelSet := make(map[string]struct{})
	for _, name := range GetEnabledModels() {
		modelSet[name] = struct{}{}
	}
	var abilityModels []string
	if err := DB.Model(&Ability{}).Where("enabled = ?", true).Distinct("model").Pluck("model", &abilityModels).Error; err != nil {
		return nil, err
	}
	for _, name := range abilityModels {
		modelSet[name] = struct{}{}
	}
	if len(modelSet) == 0 {
		return []string{}, nil
	}
	models := make([]string, 0, len(modelSet))
	for name := range modelSet {
		models = append(models, name)
	}

	// 2. 查询已有的元数据模型名
	var existing []string
	if err := DB.Model(&Model{}).Where("model_name IN ?", models).Pluck("model_name", &existing).Error; err != nil {
		return nil, err
	}

	existingSet := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		existingSet[e] = struct{}{}
	}

	// 3. 收集缺失模型
	var missing []string
	for _, name := range models {
		if _, ok := existingSet[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing, nil
}
