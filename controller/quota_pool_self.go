package controller

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func selfQuotaPool(c *gin.Context) (*model.QuotaPool, *model.QuotaPoolAdminSummary, bool) {
	if !requireQuotaPoolFeature(c) {
		return nil, nil, false
	}
	admin, err := model.GetQuotaPoolAdminSummary(c.GetInt("id"))
	if err != nil || admin == nil {
		if err == nil {
			err = model.ErrQuotaPoolPermissionDenied
		}
		writeQuotaPoolError(c, err)
		return nil, nil, false
	}
	pool, err := model.GetQuotaPoolById(admin.PoolId)
	if err != nil {
		writeQuotaPoolError(c, err)
		return nil, nil, false
	}
	return pool, admin, true
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
	common.ApiSuccess(c, gin.H{
		"pool": pool, "admin": admin,
		"capabilities":               service.ResolveQuotaPoolCapabilities(c.GetInt("role"), admin.Level),
		"weekly_auto_recharge_usage": weeklyUsage,
	})
}

func UpdateSelfQuotaPool(c *gin.Context) {
	pool, _, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	var req quotaPoolUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		writeQuotaPoolError(c, model.ErrQuotaPoolInvalidAmount)
		return
	}
	updates := map[string]any{}
	if req.AutoRechargeAmount != nil {
		switch {
		case *req.AutoRechargeAmount < 0:
			updates["auto_recharge_amount"] = model.QuotaPoolAutoRechargeInherit
		case *req.AutoRechargeAmount == 0:
			updates["auto_recharge_amount"] = model.QuotaPoolAutoRechargeOff
		default:
			updates["auto_recharge_amount"] = quotaAmountToInternal(*req.AutoRechargeAmount)
		}
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
	common.ApiSuccess(c, nil)
}

func GetSelfQuotaPoolMembers(c *gin.Context) {
	pool, _, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	page := common.GetPageQuery(c)
	items, total, err := model.ListQuotaPoolMembers(pool.Id, page)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
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
	stats, err := model.GetQuotaPoolStats(pool.Id, 0, 0)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func GetSelfQuotaPoolCandidates(c *gin.Context) {
	if _, _, ok := selfQuotaPool(c); !ok {
		return
	}
	GetQuotaPoolCandidates(c)
}

func AddSelfQuotaPoolMember(c *gin.Context) {
	pool, _, ok := selfQuotaPool(c)
	if !ok {
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
	pool, _, ok := selfQuotaPool(c)
	if !ok {
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
	pool, _, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	userId, ok := parseQuotaPoolUserID(c)
	if !ok {
		return
	}
	change, err := model.ReclaimQuotaToPool(pool.Id, userId, quotaPoolRechargeAmount(pool), c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, pool.Id, "quota_pool.member_reclaim", map[string]any{"user_id": userId, "amount": change.Amount})
	common.ApiSuccess(c, change)
}

func GrantSelfQuotaPoolAdmin(c *gin.Context) {
	pool, admin, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	if !service.ResolveQuotaPoolCapabilities(c.GetInt("role"), admin.Level).CanManageV1Admins {
		writeQuotaPoolError(c, model.ErrQuotaPoolPermissionDenied)
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
	common.ApiSuccess(c, nil)
}

func RevokeSelfQuotaPoolAdmin(c *gin.Context) {
	pool, admin, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	if !service.ResolveQuotaPoolCapabilities(c.GetInt("role"), admin.Level).CanManageV1Admins {
		writeQuotaPoolError(c, model.ErrQuotaPoolPermissionDenied)
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
	common.ApiSuccess(c, nil)
}
