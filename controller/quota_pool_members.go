package controller

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetQuotaPoolMembers(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	page := common.GetPageQuery(c)
	items, total, err := model.ListQuotaPoolMembers(id, c.Query("keyword"), page)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	pool, err := model.GetQuotaPoolById(id)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	attachQuotaPoolMemberReclaimAmounts(pool, items)
	quotaPoolPage(c, items, total)
}

func attachQuotaPoolMemberReclaimAmounts(pool *model.QuotaPool, items []model.QuotaPoolMember) {
	for i := range items {
		items[i].ReclaimAmounts = quotaPoolReclaimAmounts(pool, items[i].Quota)
	}
}

func GetQuotaPoolCandidates(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	page := common.GetPageQuery(c)
	items, total, err := model.ListQuotaPoolCandidates(c.Query("keyword"), page)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	quotaPoolPage(c, items, total)
}

func AddQuotaPoolMember(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	var req quotaPoolMemberRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || req.UserId <= 0 {
		writeQuotaPoolError(c, model.ErrQuotaPoolMemberMismatch)
		return
	}
	result, err := model.AddUserToQuotaPool(req.UserId, id, c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, id, "quota_pool.member_add", map[string]any{"user_id": req.UserId})
	recharge := service.TryAutoRechargeUserById(req.UserId)
	common.ApiSuccess(c, gin.H{"move": result, "initial_recharge": recharge})
}

func RemoveQuotaPoolMember(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	userId, ok := parseQuotaPoolUserID(c)
	if !ok {
		return
	}
	removeQuotaPoolMember(c, id, userId, true)
}

func removeQuotaPoolMember(c *gin.Context, sourcePoolId, userId int, allowAdminRemoval bool) {
	result, err := model.RemoveQuotaPoolMember(model.QuotaPoolMemberRemoval{
		SourcePoolId: sourcePoolId, UserId: userId, OperatorId: c.GetInt("id"),
		AllowAdminRemoval: allowAdminRemoval,
	})
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, sourcePoolId, "quota_pool.member_remove", map[string]any{
		"user_id":          userId,
		"target_pool_id":   result.NewPoolId,
		"reclaimed_amount": result.Change.Amount,
		"admin_revoked":    result.AdminRevoked,
	})
	common.ApiSuccess(c, result)
}

func MoveUserQuotaPool(c *gin.Context) {
	userId, ok := parseQuotaPoolUserID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	var req quotaPoolMoveRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || req.PoolId < 0 {
		writeQuotaPoolError(c, model.ErrQuotaPoolNotFound)
		return
	}
	result, err := model.MoveUserBetweenQuotaPools(userId, req.PoolId, true, c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, req.PoolId, "quota_pool.member_move", map[string]any{"user_id": userId})
	common.ApiSuccess(c, result)
}

func RechargeQuotaPoolMember(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	userId, ok := parseQuotaPoolUserID(c)
	if !ok {
		return
	}
	pool, err := model.GetQuotaPoolById(id)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	change, err := model.AllocateQuotaFromPool(id, userId, quotaPoolRechargeAmount(pool), model.QuotaPoolTransactionAllocateManual, c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, id, "quota_pool.member_recharge", map[string]any{"user_id": userId, "amount": -change.Amount})
	common.ApiSuccess(c, change)
}

func ReclaimQuotaPoolMember(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	userId, ok := parseQuotaPoolUserID(c)
	if !ok {
		return
	}
	pool, err := model.GetQuotaPoolById(id)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	reclaimQuotaPoolMember(c, pool, userId)
}

func reclaimQuotaPoolMember(c *gin.Context, pool *model.QuotaPool, userId int) {
	var req struct {
		Amount *int `json:"amount"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || req.Amount == nil {
		writeQuotaPoolError(c, model.ErrQuotaPoolInvalidAmount)
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	allowed := quotaPoolReclaimAmounts(pool, user.Quota)
	if err := validateQuotaPoolReclaimAmount(allowed, *req.Amount); err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	change, err := model.ReclaimQuotaToPool(pool.Id, userId, *req.Amount, c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, pool.Id, "quota_pool.member_reclaim", map[string]any{"user_id": userId, "amount": change.Amount})
	common.ApiSuccess(c, change)
}

func validateQuotaPoolReclaimAmount(allowedAmounts []int, amount int) error {
	for _, allowed := range allowedAmounts {
		if amount == allowed {
			return nil
		}
	}
	return model.ErrQuotaPoolInvalidAmount
}

func GrantQuotaPoolAdmin(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	var req quotaPoolAdminRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		writeQuotaPoolError(c, model.ErrQuotaPoolPermissionDenied)
		return
	}
	if err := model.GrantQuotaPoolAdmin(id, req.UserId); err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, id, "quota_pool.admin_grant", map[string]any{"user_id": req.UserId})
	common.ApiSuccess(c, nil)
}

func RevokeQuotaPoolAdmin(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	userId, ok := parseQuotaPoolUserID(c)
	if !ok {
		return
	}
	if err := model.RevokeQuotaPoolAdmin(id, userId); err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, id, "quota_pool.admin_revoke", map[string]any{"user_id": userId})
	common.ApiSuccess(c, nil)
}

func GetQuotaPoolTransactions(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	page := common.GetPageQuery(c)
	items, total, err := model.ListQuotaPoolTransactions(id, page)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	quotaPoolPage(c, items, total)
}

func GetQuotaPoolOperationLogs(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	page := common.GetPageQuery(c)
	items, total, err := model.ListQuotaPoolOperationLogs(id, page)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	quotaPoolPage(c, items, total)
}

func GetQuotaPoolStats(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	start, end := statsRange(c, time.Now())
	stats, err := model.GetQuotaPoolStats(id, start, end)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}
