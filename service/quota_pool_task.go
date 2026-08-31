package service

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type quotaPoolMaintenanceHandler struct{}

type quotaPoolMaintenanceResult struct {
	MonthlyRefilled int            `json:"monthly_refilled"`
	UsersChecked    int            `json:"users_checked"`
	UsersRecharged  int            `json:"users_recharged"`
	UsersSkipped    int            `json:"users_skipped"`
	SkipReasons     map[string]int `json:"skip_reasons,omitempty"`
}

func (quotaPoolMaintenanceHandler) Type() string {
	return model.SystemTaskTypeQuotaPoolMaintenance
}

func (quotaPoolMaintenanceHandler) Enabled() bool {
	return operation_setting.GetAutoRechargeSetting().Enabled
}

func (quotaPoolMaintenanceHandler) Interval() time.Duration {
	minutes := operation_setting.GetAutoRechargeSetting().Interval
	if minutes <= 0 {
		minutes = 30
	}
	return time.Duration(minutes) * time.Minute
}

func (quotaPoolMaintenanceHandler) NewPayload() any { return struct{}{} }

func (quotaPoolMaintenanceHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	result := quotaPoolMaintenanceResult{SkipReasons: map[string]int{}}
	monthly, err := RefillMonthlyQuotaPools(time.Now())
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	result.MonthlyRefilled = monthly.Refilled
	var users []model.User
	if err := model.DB.Where("status = ?", common.UserStatusEnabled).Find(&users).Error; err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	for i := range users {
		select {
		case <-ctx.Done():
			return
		default:
		}
		result.UsersChecked++
		recharge := tryAutoRechargeUser(&users[i], time.Now())
		if recharge.Recharged {
			result.UsersRecharged++
		} else {
			result.UsersSkipped++
			result.SkipReasons[autoRechargeSkipReason(recharge.Reason)]++
		}
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logSystemTaskLockError(ctx, task, err)
	}
}

func autoRechargeSkipReason(reason string) string {
	switch reason {
	case "disabled", "quota_above_threshold", "quota_pool_not_found",
		"new_user_pool_disabled", "quota_pool_disabled", "amount_not_configured",
		"weekly_count_failed", "weekly_limited", "monthly_count_failed", "monthly_limited":
		return reason
	case model.ErrQuotaPoolInsufficientQuota.Error():
		return "quota_pool_insufficient"
	default:
		return "recharge_failed"
	}
}

func init() {
	RegisterSystemTaskHandler(quotaPoolMaintenanceHandler{})
}
