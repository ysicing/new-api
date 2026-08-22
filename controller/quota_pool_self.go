package controller

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func selfQuotaPool(c *gin.Context) (*model.QuotaPool, *model.QuotaPoolAdminSummary, bool) {
	if !requireQuotaPoolFeature(c) {
		return nil, nil, false
	}
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		writeQuotaPoolError(c, err)
		return nil, nil, false
	}
	admin, err := model.GetQuotaPoolAdminSummary(c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return nil, nil, false
	}
	if admin != nil {
		pool, err := model.GetQuotaPoolById(admin.PoolId)
		if err != nil {
			writeQuotaPoolError(c, err)
			return nil, nil, false
		}
		return pool, admin, true
	}
	if user.QuotaPoolId <= 0 {
		writeQuotaPoolError(c, model.ErrQuotaPoolPermissionDenied)
		return nil, nil, false
	}
	pool, err := model.GetQuotaPoolById(user.QuotaPoolId)
	if err != nil {
		writeQuotaPoolError(c, err)
		return nil, nil, false
	}
	return pool, nil, true
}

func GetSelfQuotaPool(c *gin.Context) {
	pool, admin, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	weeklyUsage, err := service.GetWeeklyAutoRechargeUsage(user, pool, time.Now())
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	contacts, err := model.ListQuotaPoolAdminContacts(pool.Id)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	poolItem, err := model.GetQuotaPoolListItemById(pool.Id)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"pool":                       poolItem,
		"admin":                      admin,
		"capabilities":               selfQuotaPoolCapabilities(c, admin),
		"weekly_auto_recharge_usage": weeklyUsage,
		"admin_contacts":             contacts,
	})
}

func UpdateSelfQuotaPool(c *gin.Context) {
	pool, admin, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	if !requireSelfQuotaPoolEdit(c, admin) {
		return
	}
	var req quotaPoolUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		writeQuotaPoolError(c, model.ErrQuotaPoolInvalidAmount)
		return
	}
	updates := map[string]any{}
	if req.AutoRechargeAmount != nil {
		amount, err := normalizeQuotaPoolAutoRechargeAmount(*req.AutoRechargeAmount)
		if err != nil {
			writeQuotaPoolError(c, err)
			return
		}
		updates["auto_recharge_amount"] = amount
	}
	if req.WeeklyLimit != nil {
		updates["weekly_limit"] = *req.WeeklyLimit
	}
	if req.MonthlyLimit != nil {
		updates["monthly_limit"] = *req.MonthlyLimit
	}
	if _, err := model.UpdateQuotaPoolConfig(pool.Id, updates, c.GetInt("id")); err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, pool.Id, "quota_pool.self_update", map[string]any{"fields": len(updates)})
	warning := ""
	if req.AutoRechargeAmount != nil && *req.AutoRechargeAmount > float64(operation_setting.GetAutoRechargeSetting().Amount*3) {
		warning = "自动充值金额超过全局默认金额的 3 倍，请确认配置风险"
	}
	quotaPoolSuccessWithMessage(c, warning, nil)
}

func GetSelfQuotaPoolMembers(c *gin.Context) {
	pool, _, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	page := common.GetPageQuery(c)
	items, total, err := model.ListQuotaPoolMembers(pool.Id, c.Query("keyword"), page)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	attachQuotaPoolMemberReclaimAmounts(pool, items)
	quotaPoolPage(c, items, total)
}

func GetSelfQuotaPoolTransactions(c *gin.Context) {
	pool, _, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	page := common.GetPageQuery(c)
	items, total, err := model.ListQuotaPoolTransactions(pool.Id, page)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	quotaPoolPage(c, items, total)
}

func GetSelfQuotaPoolOperationLogs(c *gin.Context) {
	pool, _, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	page := common.GetPageQuery(c)
	items, total, err := model.ListQuotaPoolOperationLogs(pool.Id, page)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	quotaPoolPage(c, items, total)
}

func GetSelfQuotaPoolStats(c *gin.Context) {
	pool, _, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	start, end := statsRange(c, time.Now())
	stats, err := model.GetQuotaPoolStats(pool.Id, start, end)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func GetSelfQuotaPoolCandidates(c *gin.Context) {
	_, admin, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	if !requireSelfQuotaPoolMembersManagement(c, admin) {
		return
	}
	GetQuotaPoolCandidates(c)
}

func AddSelfQuotaPoolMember(c *gin.Context) {
	pool, admin, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	if !requireSelfQuotaPoolMembersManagement(c, admin) {
		return
	}
	var req quotaPoolMemberRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		writeQuotaPoolError(c, model.ErrQuotaPoolMemberMismatch)
		return
	}
	result, err := model.MoveUserBetweenQuotaPools(req.UserId, pool.Id, false, c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, pool.Id, "quota_pool.member_add", map[string]any{"user_id": req.UserId})
	recharge := service.TryAutoRechargeUserById(req.UserId)
	common.ApiSuccess(c, gin.H{"move": result, "initial_recharge": recharge})
}

func MoveSelfQuotaPoolMember(c *gin.Context) {
	writeQuotaPoolError(c, model.ErrQuotaPoolPermissionDenied)
}

func RechargeSelfQuotaPoolMember(c *gin.Context) {
	pool, admin, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	if !requireSelfQuotaPoolMembersManagement(c, admin) {
		return
	}
	userId, ok := parseQuotaPoolUserID(c)
	if !ok {
		return
	}
	change, err := model.AllocateQuotaFromPool(pool.Id, userId, quotaPoolRechargeAmount(pool), model.QuotaPoolTransactionAllocateManual, c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, pool.Id, "quota_pool.member_recharge", map[string]any{"user_id": userId, "amount": -change.Amount})
	common.ApiSuccess(c, change)
}

func ReclaimSelfQuotaPoolMember(c *gin.Context) {
	pool, admin, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	if !requireSelfQuotaPoolMembersManagement(c, admin) {
		return
	}
	userId, ok := parseQuotaPoolUserID(c)
	if !ok {
		return
	}
	reclaimQuotaPoolMember(c, pool, userId)
}

func GrantSelfQuotaPoolAdmin(c *gin.Context) {
	pool, admin, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	if !requireSelfQuotaPoolV2AdminManagement(c, admin) {
		return
	}
	var req quotaPoolAdminRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || req.Level != model.QuotaPoolAdminLevelV1 {
		writeQuotaPoolError(c, model.ErrQuotaPoolPermissionDenied)
		return
	}
	if err := model.GrantQuotaPoolAdmin(pool.Id, req.UserId, req.Level); err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, pool.Id, "quota_pool.admin_grant", map[string]any{"user_id": req.UserId, "level": req.Level})
	common.ApiSuccess(c, gin.H{"pool_id": pool.Id})
}

func RevokeSelfQuotaPoolAdmin(c *gin.Context) {
	pool, admin, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	if !requireSelfQuotaPoolV1AdminManagement(c, admin) {
		return
	}
	userId, ok := parseQuotaPoolUserID(c)
	if !ok {
		return
	}
	if err := model.RevokeQuotaPoolAdmin(pool.Id, userId); err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, pool.Id, "quota_pool.admin_revoke", map[string]any{"user_id": userId})
	common.ApiSuccess(c, gin.H{"pool_id": pool.Id})
}
