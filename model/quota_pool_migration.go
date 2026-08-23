package model

import (
	"errors"

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
	if err := db.AutoMigrate(
		&quotaPoolUserCompatibilityColumns{},
		&QuotaPool{},
		&QuotaPoolAdmin{},
		&QuotaPoolTransaction{},
	); err != nil {
		return err
	}
	if err := db.Model(&QuotaPoolAdmin{}).
		Where("level <> ?", QuotaPoolAdminLevel).
		Update("level", QuotaPoolAdminLevel).Error; err != nil {
		return err
	}
	if !hadQuotaPoolTable || !db.Migrator().HasTable(&Option{}) {
		return nil
	}
	return persistLegacyAutoRechargeDefaults(db)
}

func persistLegacyAutoRechargeDefaults(db *gorm.DB) error {
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
		if err := ensureDefaultQuotaPool(tx); err != nil {
			return err
		}
		return ensureNewUserQuotaPool(tx)
	})
}

func SyncSystemQuotaPools() error {
	return syncSystemQuotaPools(DB)
}

func ensureDefaultQuotaPool(db *gorm.DB) error {
	var pool QuotaPool
	err := db.Where("pool_type = ? OR is_default = ?", QuotaPoolTypeDefault, true).
		Order("id ASC").First(&pool).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(&QuotaPool{
		Name:               QuotaPoolDefaultName,
		PoolType:           QuotaPoolTypeDefault,
		Enabled:            true,
		IsDefault:          true,
		BaseQuota:          QuotaPoolUnlimitedQuota,
		Quota:              QuotaPoolUnlimitedQuota,
		AutoRechargeAmount: QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       QuotaPoolAutoRechargeInherit,
		MonthlyRefillDay:   1,
	}).Error
}

func ensureNewUserQuotaPool(db *gorm.DB) error {
	var pool QuotaPool
	err := db.Where("pool_type = ?", QuotaPoolTypeNewUser).Order("id ASC").First(&pool).Error
	if err == nil {
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
