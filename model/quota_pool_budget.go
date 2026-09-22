package model

import (
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxQuotaPoolBudgetMonths = 24
const maxQuotaPoolBudgetSafeInteger = int64(9_007_199_254_740_991)

var (
	ErrQuotaPoolBudgetTagNotFound       = errors.New("quota pool budget tag not found")
	ErrQuotaPoolBudgetTagNameInvalid    = errors.New("quota pool budget tag name invalid")
	ErrQuotaPoolBudgetTagNameExists     = errors.New("quota pool budget tag name exists")
	ErrQuotaPoolBudgetTagInUse          = errors.New("quota pool budget tag is in use")
	ErrQuotaPoolBudgetMonthRangeInvalid = errors.New("quota pool budget month range invalid")
	ErrQuotaPoolBudgetAmountOverflow    = errors.New("quota pool budget amount overflow")
)

type QuotaPoolBudgetTag struct {
	Id             int    `json:"id"`
	Name           string `json:"name" gorm:"type:varchar(64)"`
	NormalizedName string `json:"-" gorm:"type:varchar(64);uniqueIndex"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type QuotaPoolBudgetTagListItem struct {
	QuotaPoolBudgetTag
	PoolCount int64 `json:"pool_count"`
}

type QuotaPoolBudgetAmounts struct {
	NetRecharge    int64 `json:"net_recharge"`
	NetConsumption int64 `json:"net_consumption"`
}

func (amounts *QuotaPoolBudgetAmounts) add(delta QuotaPoolBudgetAmounts) error {
	if delta.NetRecharge > maxQuotaPoolBudgetSafeInteger || delta.NetRecharge < -maxQuotaPoolBudgetSafeInteger ||
		delta.NetConsumption > maxQuotaPoolBudgetSafeInteger || delta.NetConsumption < -maxQuotaPoolBudgetSafeInteger {
		return ErrQuotaPoolBudgetAmountOverflow
	}
	if delta.NetRecharge > 0 && amounts.NetRecharge > maxQuotaPoolBudgetSafeInteger-delta.NetRecharge ||
		delta.NetRecharge < 0 && amounts.NetRecharge < -maxQuotaPoolBudgetSafeInteger-delta.NetRecharge ||
		delta.NetConsumption > 0 && amounts.NetConsumption > maxQuotaPoolBudgetSafeInteger-delta.NetConsumption ||
		delta.NetConsumption < 0 && amounts.NetConsumption < -maxQuotaPoolBudgetSafeInteger-delta.NetConsumption {
		return ErrQuotaPoolBudgetAmountOverflow
	}
	amounts.NetRecharge += delta.NetRecharge
	amounts.NetConsumption += delta.NetConsumption
	return nil
}

type QuotaPoolBudgetMonthStat struct {
	Month string `json:"month"`
	QuotaPoolBudgetAmounts
}

type QuotaPoolBudgetPoolStat struct {
	PoolId   int    `json:"pool_id"`
	PoolName string `json:"pool_name"`
	QuotaPoolBudgetAmounts
	Months []QuotaPoolBudgetMonthStat `json:"months"`
}

type QuotaPoolBudgetTagStat struct {
	TagId     int    `json:"tag_id"`
	Name      string `json:"name"`
	PoolCount int    `json:"pool_count"`
	QuotaPoolBudgetAmounts
	Months []QuotaPoolBudgetMonthStat `json:"months"`
	Pools  []QuotaPoolBudgetPoolStat  `json:"pools"`
}

type QuotaPoolBudgetStats struct {
	StartMonth string                     `json:"start_month"`
	EndMonth   string                     `json:"end_month"`
	TimeZone   string                     `json:"time_zone"`
	Summary    QuotaPoolBudgetAmounts     `json:"summary"`
	Months     []QuotaPoolBudgetMonthStat `json:"months"`
	Tags       []QuotaPoolBudgetTagStat   `json:"tags"`
}

func normalizeQuotaPoolBudgetTagName(name string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return "", "", ErrQuotaPoolBudgetTagNameInvalid
	}
	return name, strings.ToLower(name), nil
}

func CreateQuotaPoolBudgetTag(name string) (*QuotaPoolBudgetTag, error) {
	name, normalizedName, err := normalizeQuotaPoolBudgetTagName(name)
	if err != nil {
		return nil, err
	}
	tag := &QuotaPoolBudgetTag{Name: name, NormalizedName: normalizedName}
	result := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "normalized_name"}},
		DoNothing: true,
	}).Create(tag)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrQuotaPoolBudgetTagNameExists
	}
	return tag, nil
}

func UpdateQuotaPoolBudgetTag(id int, name string) (*QuotaPoolBudgetTag, string, error) {
	name, normalizedName, err := normalizeQuotaPoolBudgetTagName(name)
	if err != nil {
		return nil, "", err
	}
	var tag QuotaPoolBudgetTag
	previousName := ""
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&tag, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuotaPoolBudgetTagNotFound
			}
			return err
		}
		previousName = tag.Name
		if tag.Name == name && tag.NormalizedName == normalizedName {
			return nil
		}
		var count int64
		if err := tx.Model(&QuotaPoolBudgetTag{}).Where("normalized_name = ? AND id <> ?", normalizedName, id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrQuotaPoolBudgetTagNameExists
		}
		result := tx.Model(&QuotaPoolBudgetTag{}).Where("id = ?", id).
			Updates(map[string]any{"name": name, "normalized_name": normalizedName})
		if result.Error != nil {
			return result.Error
		}
		tag.Name = name
		tag.NormalizedName = normalizedName
		return nil
	})
	if err != nil {
		var duplicateCount int64
		if countErr := DB.Model(&QuotaPoolBudgetTag{}).
			Where("normalized_name = ? AND id <> ?", normalizedName, id).
			Count(&duplicateCount).Error; countErr == nil && duplicateCount > 0 {
			return nil, "", ErrQuotaPoolBudgetTagNameExists
		}
		return nil, "", err
	}
	return &tag, previousName, nil
}

func DeleteQuotaPoolBudgetTag(id int) (*QuotaPoolBudgetTag, error) {
	var deleted QuotaPoolBudgetTag
	err := DB.Transaction(func(tx *gorm.DB) error {
		var tag QuotaPoolBudgetTag
		if err := lockForUpdate(tx).First(&tag, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuotaPoolBudgetTagNotFound
			}
			return err
		}
		var count int64
		if err := tx.Model(&QuotaPool{}).Where("budget_tag_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrQuotaPoolBudgetTagInUse
		}
		if err := tx.Delete(&tag).Error; err != nil {
			return err
		}
		deleted = tag
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &deleted, nil
}

func SetQuotaPoolBudgetTag(poolId, tagId int) (int, error) {
	previousTagId := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		if tagId > 0 {
			var tag QuotaPoolBudgetTag
			if err := lockForUpdate(tx).First(&tag, tagId).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrQuotaPoolBudgetTagNotFound
				}
				return err
			}
		}
		var pool QuotaPool
		if err := lockForUpdate(tx).First(&pool, poolId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuotaPoolNotFound
			}
			return err
		}
		previousTagId = pool.BudgetTagId
		return tx.Model(&pool).Update("budget_tag_id", tagId).Error
	})
	return previousTagId, err
}

type QuotaPoolBudgetAssignmentChange struct {
	Before map[int]int `json:"before"`
	After  map[int]int `json:"after"`
}

func ReplaceQuotaPoolBudgetTagPools(tagId int, poolIds []int) (*QuotaPoolBudgetAssignmentChange, error) {
	change := &QuotaPoolBudgetAssignmentChange{Before: map[int]int{}, After: map[int]int{}}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var tag QuotaPoolBudgetTag
		if err := lockForUpdate(tx).First(&tag, tagId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuotaPoolBudgetTagNotFound
			}
			return err
		}
		uniquePoolIds := make([]int, 0, len(poolIds))
		seen := make(map[int]struct{}, len(poolIds))
		for _, poolId := range poolIds {
			if poolId <= 0 {
				return ErrQuotaPoolNotFound
			}
			if _, exists := seen[poolId]; exists {
				continue
			}
			seen[poolId] = struct{}{}
			uniquePoolIds = append(uniquePoolIds, poolId)
		}
		if len(uniquePoolIds) > 0 {
			var count int64
			if err := tx.Model(&QuotaPool{}).Where("id IN ?", uniquePoolIds).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(uniquePoolIds)) {
				return ErrQuotaPoolNotFound
			}
		}
		var affected []QuotaPool
		query := tx.Model(&QuotaPool{}).Where("budget_tag_id = ?", tagId)
		if len(uniquePoolIds) > 0 {
			query = tx.Model(&QuotaPool{}).Where("budget_tag_id = ? OR id IN ?", tagId, uniquePoolIds)
		}
		if err := lockForUpdate(query).Find(&affected).Error; err != nil {
			return err
		}
		for _, pool := range affected {
			change.Before[pool.Id] = pool.BudgetTagId
			change.After[pool.Id] = pool.BudgetTagId
		}
		if err := tx.Model(&QuotaPool{}).Where("budget_tag_id = ?", tagId).Update("budget_tag_id", 0).Error; err != nil {
			return err
		}
		for poolId := range change.After {
			if change.Before[poolId] == tagId {
				change.After[poolId] = 0
			}
		}
		if len(uniquePoolIds) == 0 {
			return nil
		}
		if err := tx.Model(&QuotaPool{}).Where("id IN ?", uniquePoolIds).Update("budget_tag_id", tagId).Error; err != nil {
			return err
		}
		for _, poolId := range uniquePoolIds {
			change.After[poolId] = tagId
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return change, nil
}

func ListQuotaPoolBudgetTags() ([]QuotaPoolBudgetTagListItem, error) {
	var tags []QuotaPoolBudgetTag
	if err := DB.Order("id ASC").Find(&tags).Error; err != nil {
		return nil, err
	}
	counts := make(map[int]int64, len(tags))
	if len(tags) > 0 {
		var rows []struct {
			BudgetTagId int
			Count       int64
		}
		if err := DB.Model(&QuotaPool{}).
			Select("budget_tag_id, COUNT(*) AS count").
			Where("budget_tag_id > 0").Group("budget_tag_id").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			counts[row.BudgetTagId] = row.Count
		}
	}
	items := make([]QuotaPoolBudgetTagListItem, 0, len(tags))
	for _, tag := range tags {
		items = append(items, QuotaPoolBudgetTagListItem{QuotaPoolBudgetTag: tag, PoolCount: counts[tag.Id]})
	}
	return items, nil
}

func GetQuotaPoolBudgetStats(startMonth, endMonth string, location *time.Location) (*QuotaPoolBudgetStats, error) {
	if location == nil {
		location = common.BeijingTimeLocation
	}
	start, err := time.ParseInLocation("2006-01", startMonth, location)
	if err != nil {
		return nil, ErrQuotaPoolBudgetMonthRangeInvalid
	}
	end, err := time.ParseInLocation("2006-01", endMonth, location)
	if err != nil || end.Before(start) {
		return nil, ErrQuotaPoolBudgetMonthRangeInvalid
	}
	monthStarts := make([]time.Time, 0, maxQuotaPoolBudgetMonths)
	for month := start; !month.After(end); month = month.AddDate(0, 1, 0) {
		monthStarts = append(monthStarts, month)
		if len(monthStarts) > maxQuotaPoolBudgetMonths {
			return nil, ErrQuotaPoolBudgetMonthRangeInvalid
		}
	}

	tags, err := ListQuotaPoolBudgetTags()
	if err != nil {
		return nil, err
	}
	var pools []QuotaPool
	if err := DB.Where("budget_tag_id > 0").Order("budget_tag_id ASC, id ASC").Find(&pools).Error; err != nil {
		return nil, err
	}
	stats := &QuotaPoolBudgetStats{
		StartMonth: startMonth, EndMonth: endMonth, TimeZone: location.String(),
		Months: make([]QuotaPoolBudgetMonthStat, len(monthStarts)),
		Tags:   make([]QuotaPoolBudgetTagStat, len(tags)),
	}
	tagIndex := make(map[int]int, len(tags))
	poolPosition := make(map[int][2]int, len(pools))
	for index, month := range monthStarts {
		stats.Months[index].Month = month.Format("2006-01")
	}
	for index, tag := range tags {
		tagIndex[tag.Id] = index
		stats.Tags[index] = QuotaPoolBudgetTagStat{
			TagId: tag.Id, Name: tag.Name, Months: make([]QuotaPoolBudgetMonthStat, len(monthStarts)),
		}
		for monthIndex, month := range monthStarts {
			stats.Tags[index].Months[monthIndex].Month = month.Format("2006-01")
		}
	}
	poolIds := make([]int, 0, len(pools))
	for _, pool := range pools {
		tagPosition, exists := tagIndex[pool.BudgetTagId]
		if !exists {
			continue
		}
		poolPosition[pool.Id] = [2]int{tagPosition, len(stats.Tags[tagPosition].Pools)}
		poolIds = append(poolIds, pool.Id)
		stats.Tags[tagPosition].Pools = append(stats.Tags[tagPosition].Pools, QuotaPoolBudgetPoolStat{
			PoolId: pool.Id, PoolName: pool.Name, Months: make([]QuotaPoolBudgetMonthStat, len(monthStarts)),
		})
		stats.Tags[tagPosition].PoolCount++
		for monthIndex, month := range monthStarts {
			poolIndex := len(stats.Tags[tagPosition].Pools) - 1
			stats.Tags[tagPosition].Pools[poolIndex].Months[monthIndex].Month = month.Format("2006-01")
		}
	}
	if len(poolIds) == 0 {
		return stats, nil
	}

	types := []string{
		QuotaPoolTransactionInitialFund, QuotaPoolTransactionManualRefill,
		QuotaPoolTransactionMonthlyRefill, QuotaPoolTransactionAdjustBase,
		QuotaPoolTransactionAllocateAuto, QuotaPoolTransactionAllocateManual,
		QuotaPoolTransactionReclaimUser,
	}
	for monthIndex, monthStart := range monthStarts {
		var rows []struct {
			PoolId int
			Type   string
			Amount int64
		}
		if err := DB.Model(&QuotaPoolTransaction{}).
			Select("pool_id, type, SUM(amount) AS amount").
			Where("pool_id IN ? AND type IN ? AND created_at >= ? AND created_at < ?", poolIds, types, monthStart.Unix(), monthStart.AddDate(0, 1, 0).Unix()).
			Where("(type NOT IN ? OR amount < 0) AND (type <> ? OR amount > 0)", []string{QuotaPoolTransactionAllocateAuto, QuotaPoolTransactionAllocateManual}, QuotaPoolTransactionReclaimUser).
			Group("pool_id, type").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			position, exists := poolPosition[row.PoolId]
			if !exists {
				continue
			}
			amounts := QuotaPoolBudgetAmounts{}
			if row.Amount > maxQuotaPoolBudgetSafeInteger || row.Amount < -maxQuotaPoolBudgetSafeInteger {
				return nil, ErrQuotaPoolBudgetAmountOverflow
			}
			switch row.Type {
			case QuotaPoolTransactionInitialFund, QuotaPoolTransactionManualRefill, QuotaPoolTransactionMonthlyRefill, QuotaPoolTransactionAdjustBase:
				amounts.NetRecharge = row.Amount
			case QuotaPoolTransactionAllocateAuto, QuotaPoolTransactionAllocateManual, QuotaPoolTransactionReclaimUser:
				amounts.NetConsumption = -row.Amount
			}
			tag := &stats.Tags[position[0]]
			pool := &tag.Pools[position[1]]
			for _, target := range []*QuotaPoolBudgetAmounts{
				&pool.QuotaPoolBudgetAmounts,
				&pool.Months[monthIndex].QuotaPoolBudgetAmounts,
				&tag.QuotaPoolBudgetAmounts,
				&tag.Months[monthIndex].QuotaPoolBudgetAmounts,
				&stats.Summary,
				&stats.Months[monthIndex].QuotaPoolBudgetAmounts,
			} {
				if err := target.add(amounts); err != nil {
					return nil, err
				}
			}
		}
	}
	for index := range stats.Tags {
		sort.Slice(stats.Tags[index].Pools, func(left, right int) bool {
			return stats.Tags[index].Pools[left].PoolId < stats.Tags[index].Pools[right].PoolId
		})
	}
	return stats, nil
}
