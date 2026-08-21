package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
	common.ApiSuccess(c, gin.H{"move": result})
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
	change, err := model.ReclaimQuotaToPool(id, userId, amount, c.GetInt("id"))
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	common.ApiSuccess(c, change)
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
	common.ApiSuccess(c, gin.H{"items": []any{}, "total": 0})
}

func GetQuotaPoolStats(c *gin.Context) {
	common.ApiSuccess(c, gin.H{"usage": []any{}, "recharge": []any{}})
}
