package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type quotaPoolUserCompatibilityColumns struct {
	Id          int    `gorm:"primaryKey"`
	QuotaPoolId int    `gorm:"type:int;default:0;column:quota_pool_id;index"`
	LDAPId      string `gorm:"type:varchar(256);column:ldap_id;index"`
	Department  string `gorm:"type:varchar(512);column:department;index"`
}

func (quotaPoolUserCompatibilityColumns) TableName() string { return "users" }

func migrateQuotaPoolSchema(db *gorm.DB) error {
	hadQuotaPoolTable := db.Migrator().HasTable(&QuotaPool{})
	hadEnabledColumn := hadQuotaPoolTable && db.Migrator().HasColumn(&QuotaPool{}, "enabled")
	if err := db.AutoMigrate(
		&quotaPoolUserCompatibilityColumns{},
		&QuotaPool{},
		&QuotaPoolBudgetTag{},
		&QuotaPoolAdmin{},
		&QuotaPoolTransaction{},
	); err != nil {
		return err
	}
	// 仅为没有状态列的旧表补业务默认值，绝不覆盖旧表已保存的 false。
	if hadQuotaPoolTable && !hadEnabledColumn {
		if err := db.Model(&QuotaPool{}).Where("1 = 1").Update("enabled", true).Error; err != nil {
			return err
		}
	}
	if err := db.Model(&QuotaPoolAdmin{}).
		Where("level <> ?", QuotaPoolAdminLevel).
		Update("level", QuotaPoolAdminLevel).Error; err != nil {
		return err
	}
	if !db.Migrator().HasTable(&Option{}) {
		return nil
	}
	return persistLegacyAutoRechargeDefaults(db, hadQuotaPoolTable)
}

func persistLegacyAutoRechargeDefaults(db *gorm.DB, legacy bool) error {
	var count int64
	if err := db.Model(&Option{}).
		Where(map[string]any{"key": "auto_recharge_setting.enabled"}).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	defaults := []Option{
		{Key: "auto_recharge_setting.enabled", Value: "true"},
		{Key: "auto_recharge_setting.interval", Value: "30"},
		{Key: "auto_recharge_setting.threshold", Value: "50"},
		{Key: "auto_recharge_setting.amount", Value: "200"},
		{Key: "auto_recharge_setting.weekly_limit", Value: "0"},
		{Key: "auto_recharge_setting.monthly_limit", Value: "0"},
	}
	// 新安装必须持久化关闭状态，否则第二次启动会被误判为尚未迁移的 rc25。
	if !legacy {
		defaults = []Option{{Key: "auto_recharge_setting.enabled", Value: "false"}}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		upsert := clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoNothing: true,
		}
		for i := range defaults {
			if err := tx.Clauses(upsert).Create(&defaults[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func syncSystemQuotaPools(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := migrateLegacyQuotaPools(tx); err != nil {
			return err
		}
		return ensureNewUserQuotaPool(tx)
	})
}

func SyncSystemQuotaPools() error {
	return syncSystemQuotaPools(DB)
}

// migrateLegacyQuotaPools 将存量池及虚拟成员归属迁为普通池。调用方必须处于事务中。
// 仅首次转换无限余额；后续同步不得覆盖管理员配置或重新创建已删除的池。
func migrateLegacyQuotaPools(tx *gorm.DB) error {
	var pools []QuotaPool
	if err := lockForUpdate(tx).
		Where("pool_type <> ? AND (pool_type = ? OR is_default = ? OR legacy_default = ?)", QuotaPoolTypeNewUser, QuotaPoolTypeDefault, true, true).
		Order("id ASC").Find(&pools).Error; err != nil {
		return err
	}
	if len(pools) == 0 {
		var count int64
		if err := tx.Model(&quotaPoolUserCompatibilityColumns{}).Where("quota_pool_id = ?", QuotaPoolDefaultUserPoolId).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		pool := QuotaPool{
			Name: QuotaPoolDefaultName, PoolType: QuotaPoolTypeNormal, Enabled: true,
			LegacyDefault: true, BaseQuota: 0, Quota: 0,
			AutoRechargeAmount: QuotaPoolAutoRechargeInherit,
			WeeklyLimit:        QuotaPoolAutoRechargeInherit, MonthlyLimit: QuotaPoolAutoRechargeInherit,
			MonthlyRefillDay: 1,
		}
		if err := tx.Create(&pool).Error; err != nil {
			return err
		}
		pools = append(pools, pool)
	}
	for _, pool := range pools {
		if pool.PoolType == QuotaPoolTypeNormal && !pool.IsDefault && pool.LegacyDefault {
			continue
		}
		if pool.BaseQuota < QuotaPoolUnlimitedQuota || pool.BaseQuota > common.MaxWalletQuota ||
			pool.Quota < QuotaPoolUnlimitedQuota || pool.Quota > common.MaxWalletQuota {
			return ErrQuotaPoolInvalidAmount
		}
		updates := map[string]any{"pool_type": QuotaPoolTypeNormal, "is_default": false, "legacy_default": true}
		// -1 代表历史无限额度，并非负债；不从成员余额扣除，也不伪造历史拨付流水。
		if pool.BaseQuota == QuotaPoolUnlimitedQuota {
			updates["base_quota"] = 0
		}
		if pool.Quota == QuotaPoolUnlimitedQuota {
			updates["quota"] = 0
		}
		if err := tx.Model(&QuotaPool{}).Where("id = ?", pool.Id).Updates(updates).Error; err != nil {
			return err
		}
	}
	// 兼容表模型不带软删除过滤，已注销用户恢复后也必须使用真实池 ID。
	return tx.Model(&quotaPoolUserCompatibilityColumns{}).
		Where("quota_pool_id = ?", QuotaPoolDefaultUserPoolId).Update("quota_pool_id", pools[0].Id).Error
}

func ensureNewUserQuotaPool(db *gorm.DB) error {
	var pool QuotaPool
	err := db.Where("pool_type = ?", QuotaPoolTypeNewUser).Order("id ASC").First(&pool).Error
	if err == nil {
		if pool.Name == quotaPoolLegacyNewUserName {
			return db.Model(&pool).Update("name", QuotaPoolNewUserName).Error
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(&QuotaPool{
		Name:               QuotaPoolNewUserName,
		PoolType:           QuotaPoolTypeNewUser,
		Enabled:            true,
		BaseQuota:          QuotaPoolUnlimitedQuota,
		Quota:              QuotaPoolUnlimitedQuota,
		AutoRechargeAmount: QuotaPoolAutoRechargeOff,
		WeeklyLimit:        QuotaPoolAutoRechargeOff,
		MonthlyLimit:       QuotaPoolAutoRechargeOff,
		MonthlyRefillDay:   1,
	}).Error
}
