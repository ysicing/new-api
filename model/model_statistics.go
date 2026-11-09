package model

type ModelUsageStat struct {
	ModelName string  `json:"model_name" gorm:"column:model_name"`
	Count     int64   `json:"count" gorm:"column:count"`
	Quota     int64   `json:"quota" gorm:"column:quota"`
	Share     float64 `json:"share" gorm:"-"`
}

func GetModelStatistics(startTimestamp, endTimestamp int64, userId int) ([]ModelUsageStat, error) {
	query := DB.Table("quota_data").
		Select("model_name, COALESCE(SUM(count), 0) AS count, COALESCE(SUM(quota), 0) AS quota").
		Where("model_name <> ''")
	if startTimestamp > 0 {
		query = query.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	var items []ModelUsageStat
	if err := query.Group("model_name").
		Order("quota DESC, count DESC, model_name ASC").
		Scan(&items).Error; err != nil {
		return nil, err
	}
	var totalQuota int64
	for _, item := range items {
		totalQuota += item.Quota
	}
	if totalQuota > 0 {
		for i := range items {
			items[i].Share = float64(items[i].Quota) / float64(totalQuota)
		}
	}
	return items, nil
}
