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
	items, total, err := model.ListQuotaPoolMembers(id, page)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	quotaPoolPage(c, items, total)
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
	result, err := model.MoveUserBetweenQuotaPools(req.UserId, id, false, c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, id, "quota_pool.member_add", map[string]any{"user_id": req.UserId})
	recharge := service.TryAutoRechargeUserById(req.UserId)
	common.ApiSuccess(c, gin.H{"move": result, "initial_recharge": recharge})
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
	amount := quotaPoolRechargeAmount(pool)
	var req struct {
		Amount *int `json:"amount"`
	}
	if c.Request.ContentLength > 0 {
		if err := common.DecodeJson(c.Request.Body, &req); err != nil {
			writeQuotaPoolError(c, model.ErrQuotaPoolInvalidAmount)
			return
		}
	}
	if req.Amount != nil {
		amount = *req.Amount
	}
	if err := validateQuotaPoolReclaimAmount(quotaPoolRechargeAmount(pool), amount); err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	change, err := model.ReclaimQuotaToPool(id, userId, amount, c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, id, "quota_pool.member_reclaim", map[string]any{"user_id": userId, "amount": change.Amount})
	common.ApiSuccess(c, change)
}

func validateQuotaPoolReclaimAmount(baseAmount, amount int) error {
	if baseAmount <= 0 || amount <= 0 {
		return model.ErrQuotaPoolInvalidAmount
	}
	for _, factor := range []int{100, 50, 40, 30, 20, 10} {
		if amount == baseAmount*factor/100 {
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
	if err := model.GrantQuotaPoolAdmin(id, req.UserId, req.Level); err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, id, "quota_pool.admin_grant", map[string]any{"user_id": req.UserId, "level": req.Level})
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
	start, end := quotaPoolStatsRange(c, time.Now())
	stats, err := model.GetQuotaPoolStats(id, start, end)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}
