package controller

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestGetManageRechargeQuotaAmount(t *testing.T) {
	cfg := operation_setting.GetAutoRechargeSetting()
	original := cfg.Amount
	defer func() {
		cfg.Amount = original
	}()

	cfg.Amount = 0
	_, err := getManageRechargeQuotaAmount()
	if err == nil {
		t.Fatalf("expected error when amount <= 0")
	}
	if !strings.Contains(err.Error(), "自动充值金额未配置或小于等于0") {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg.Amount = 3
	quota, err := getManageRechargeQuotaAmount()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := int(float64(3) * common.QuotaPerUnit)
	if quota != want {
		t.Fatalf("unexpected quota, got %d want %d", quota, want)
	}
}

func TestFormatAdminTempQuotaLog(t *testing.T) {
	adminID := 42
	amountQuota := int(2 * common.QuotaPerUnit)

	got := formatAdminTempQuotaLog(adminID, amountQuota)
	want := fmt.Sprintf("管理员(ID:%d)添加%s临时额度", adminID, logger.LogQuota(amountQuota))
	if got != want {
		t.Fatalf("unexpected log content, got %q want %q", got, want)
	}
}
