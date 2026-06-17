package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOptionTestDB(t *testing.T) {
	t.Helper()

	oldDB := DB
	oldOptionMap := common.OptionMap
	oldDataExportEnabled := common.DataExportEnabled
	oldQuotaPoolEnabled := common.QuotaPoolEnabled

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	if err := db.AutoMigrate(&Option{}); err != nil {
		t.Fatalf("failed to migrate option table: %v", err)
	}
	common.OptionMap = map[string]string{}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
		common.OptionMap = oldOptionMap
		common.DataExportEnabled = oldDataExportEnabled
		common.QuotaPoolEnabled = oldQuotaPoolEnabled
	})
}

func TestUpdateOptionDataExportEnabledAlwaysTrue(t *testing.T) {
	setupOptionTestDB(t)

	common.DataExportEnabled = true
	if err := UpdateOption("DataExportEnabled", "false"); err != nil {
		t.Fatalf("failed to update option: %v", err)
	}

	if !common.DataExportEnabled {
		t.Fatalf("expected DataExportEnabled runtime value to remain true")
	}
	if common.OptionMap["DataExportEnabled"] != "true" {
		t.Fatalf("expected option map to store true, got %q", common.OptionMap["DataExportEnabled"])
	}

	var option Option
	if err := DB.First(&option, "key = ?", "DataExportEnabled").Error; err != nil {
		t.Fatalf("failed to load option: %v", err)
	}
	if option.Value != "true" {
		t.Fatalf("expected database option to store true, got %q", option.Value)
	}
}

func TestUpdateOptionQuotaPoolEnabledCannotTurnOffAfterEnabled(t *testing.T) {
	setupOptionTestDB(t)

	common.QuotaPoolEnabled = false
	if err := UpdateOption("QuotaPoolEnabled", "true"); err != nil {
		t.Fatalf("failed to enable quota pool option: %v", err)
	}
	if !common.QuotaPoolEnabled {
		t.Fatalf("expected quota pool runtime option to be enabled")
	}

	err := UpdateOption("QuotaPoolEnabled", "false")
	if err == nil {
		t.Fatalf("expected disabling quota pool option to be rejected")
	}
	if !common.QuotaPoolEnabled {
		t.Fatalf("expected quota pool runtime option to remain enabled")
	}

	var option Option
	if err := DB.First(&option, "key = ?", "QuotaPoolEnabled").Error; err != nil {
		t.Fatalf("failed to load option: %v", err)
	}
	if option.Value != "true" {
		t.Fatalf("expected database option to remain true, got %q", option.Value)
	}
}
