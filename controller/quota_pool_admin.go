package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func GetQuotaPools(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	items, err := model.ListQuotaPoolItems()
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items, "capabilities": currentQuotaPoolCapabilities(c, 0)})
}

func CreateQuotaPool(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	var req quotaPoolCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		writeQuotaPoolError(c, model.ErrQuotaPoolInvalidAmount)
		return
	}
	pool, err := model.CreateQuotaPool(req.Name, quotaAmountToInternal(req.BaseQuota), c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, pool.Id, "quota_pool.create", map[string]any{"name": pool.Name, "base_quota": pool.BaseQuota})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": pool})
}

func SyncDefaultQuotaPool(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	if err := model.SyncSystemQuotaPools(); err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, 0, "quota_pool.sync_system", nil)
	common.ApiSuccess(c, nil)
}

func GetQuotaPool(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	id, ok := parseQuotaPoolID(c)
	if !ok {
		return
	}
	pool, err := model.GetQuotaPoolListItemById(id)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"pool": pool, "capabilities": currentQuotaPoolCapabilities(c, 0)})
}

func UpdateQuotaPool(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	id, ok := parseQuotaPoolID(c)
	if !ok {
		return
	}
	var req quotaPoolUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		writeQuotaPoolError(c, model.ErrQuotaPoolInvalidAmount)
		return
	}
	updates, err := buildQuotaPoolUpdates(req, c.GetInt("role"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	change, err := model.UpdateQuotaPoolConfig(id, updates, c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, id, "quota_pool.update", map[string]any{"fields": len(updates)})
	warning := ""
	if req.AutoRechargeAmount != nil && *req.AutoRechargeAmount > float64(operation_setting.GetAutoRechargeSetting().Amount*3) {
		warning = "自动充值金额超过全局默认金额的 3 倍，请确认配置风险"
	}
	quotaPoolSuccessWithMessage(c, warning, change)
}

func buildQuotaPoolUpdates(req quotaPoolUpdateRequest, role int) (map[string]any, error) {
	updates := map[string]any{}
	root := role == common.RoleRootUser
	policyEditor := root || role == common.RoleQuotaPoolSuperAdmin
	if req.Name != nil || req.BaseQuota != nil || req.MonthlyRefillEnabled != nil || req.MonthlyRefillTopUp != nil || req.MonthlyRefillAmount != nil || req.MonthlyRefillDay != nil {
		if !root {
			return nil, model.ErrQuotaPoolPermissionDenied
		}
	}
	if req.AutoRechargeAmount != nil || req.WeeklyLimit != nil || req.MonthlyLimit != nil {
		if !policyEditor {
			return nil, model.ErrQuotaPoolPermissionDenied
		}
	}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.BaseQuota != nil {
		updates["base_quota"] = quotaAmountToInternal(*req.BaseQuota)
	}
	if req.AutoRechargeAmount != nil {
		amount, err := normalizeQuotaPoolAutoRechargeAmount(*req.AutoRechargeAmount)
		if err != nil {
			return nil, err
		}
		updates["auto_recharge_amount"] = amount
	}
	if req.WeeklyLimit != nil {
		updates["weekly_limit"] = *req.WeeklyLimit
	}
	if req.MonthlyLimit != nil {
		updates["monthly_limit"] = *req.MonthlyLimit
	}
	if req.MonthlyRefillEnabled != nil {
		updates["monthly_refill_enabled"] = *req.MonthlyRefillEnabled
	}
	if req.MonthlyRefillTopUp != nil {
		updates["monthly_refill_top_up"] = *req.MonthlyRefillTopUp
	}
	if req.MonthlyRefillAmount != nil {
		updates["monthly_refill_amount"] = quotaAmountToInternal(*req.MonthlyRefillAmount)
	}
	if req.MonthlyRefillDay != nil {
		if *req.MonthlyRefillDay < 1 || *req.MonthlyRefillDay > 28 {
			return nil, model.ErrQuotaPoolInvalidAmount
		}
		updates["monthly_refill_day"] = *req.MonthlyRefillDay
	}
	return updates, nil
}

func normalizeQuotaPoolAutoRechargeAmount(amount float64) (int, error) {
	if amount < float64(model.QuotaPoolAutoRechargeInherit) {
		return 0, model.ErrQuotaPoolInvalidAmount
	}
	if amount == float64(model.QuotaPoolAutoRechargeInherit) {
		return model.QuotaPoolAutoRechargeInherit, nil
	}
	if amount == 0 {
		return model.QuotaPoolAutoRechargeOff, nil
	}
	if amount <= float64(operation_setting.GetAutoRechargeSetting().Threshold) {
		return 0, model.ErrQuotaPoolInvalidAmount
	}
	return quotaAmountToInternal(amount), nil
}

func EnableQuotaPool(c *gin.Context)  { setQuotaPoolEnabled(c, true) }
func DisableQuotaPool(c *gin.Context) { setQuotaPoolEnabled(c, false) }

func setQuotaPoolEnabled(c *gin.Context, enabled bool) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	id, ok := parseQuotaPoolID(c)
	if !ok {
		return
	}
	if err := model.SetQuotaPoolEnabled(id, enabled); err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, id, "quota_pool.enabled", map[string]any{"enabled": enabled})
	common.ApiSuccess(c, nil)
}

func DeleteQuotaPool(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	id, ok := parseQuotaPoolID(c)
	if !ok {
		return
	}
	if err := model.DeleteQuotaPool(id); err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, id, "quota_pool.delete", nil)
	common.ApiSuccess(c, nil)
}

func RefillQuotaPool(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	var req quotaPoolAmountRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		writeQuotaPoolError(c, model.ErrQuotaPoolInvalidAmount)
		return
	}
	change, err := model.AddQuotaPoolManualRefill(id, quotaAmountToInternal(req.Amount), c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, id, "quota_pool.refill", map[string]any{"amount": change.Amount})
	common.ApiSuccess(c, change)
}

func quotaPoolRechargeAmount(pool *model.QuotaPool) int {
	amount := int(float64(operation_setting.GetAutoRechargeSetting().Amount) * common.QuotaPerUnit)
	if pool != nil && pool.PoolType != model.QuotaPoolTypeDefault && pool.AutoRechargeAmount >= 0 {
		amount = pool.AutoRechargeAmount
	}
	return amount
}

func quotaPoolReclaimAmounts(pool *model.QuotaPool, userQuota int) []int {
	config := operation_setting.GetAutoRechargeSetting()
	threshold := -1
	if config.Enabled {
		threshold = int(float64(config.Threshold) * common.QuotaPerUnit)
	}
	return buildQuotaPoolReclaimAmounts(quotaPoolRechargeAmount(pool), userQuota, threshold)
}

func buildQuotaPoolReclaimAmounts(baseAmount, userQuota, threshold int) []int {
	if baseAmount <= 0 || userQuota <= threshold {
		return nil
	}
	if userQuota > baseAmount && userQuota-baseAmount > threshold {
		return []int{baseAmount}
	}
	amounts := make([]int, 0, 6)
	for _, factor := range []int{100, 50, 40, 30, 20, 10} {
		amount := baseAmount * factor / 100
		if amount > 0 && userQuota-amount > threshold {
			amounts = append(amounts, amount)
		}
	}
	return amounts
}
