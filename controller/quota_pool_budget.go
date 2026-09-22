package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type quotaPoolBudgetTagRequest struct {
	Name string `json:"name"`
}

type quotaPoolBudgetTagAssignmentRequest struct {
	TagId int `json:"tag_id"`
}

type quotaPoolBudgetTagPoolsRequest struct {
	PoolIds []int `json:"pool_ids"`
}

func GetQuotaPoolBudgetTags(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	items, err := model.ListQuotaPoolBudgetTags()
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func CreateQuotaPoolBudgetTag(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	var request quotaPoolBudgetTagRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		writeQuotaPoolError(c, model.ErrQuotaPoolBudgetTagNameInvalid)
		return
	}
	tag, err := model.CreateQuotaPoolBudgetTag(request.Name)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, 0, "quota_pool.budget_tag.create", map[string]any{"tag_id": tag.Id, "name": tag.Name})
	common.ApiSuccess(c, tag)
}

func UpdateQuotaPoolBudgetTag(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("tag_id"))
	if err != nil || id <= 0 {
		writeQuotaPoolError(c, model.ErrQuotaPoolBudgetTagNotFound)
		return
	}
	var request quotaPoolBudgetTagRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		writeQuotaPoolError(c, model.ErrQuotaPoolBudgetTagNameInvalid)
		return
	}
	tag, previousName, err := model.UpdateQuotaPoolBudgetTag(id, request.Name)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, 0, "quota_pool.budget_tag.update", map[string]any{
		"tag_id": tag.Id, "before_name": previousName, "after_name": tag.Name,
	})
	common.ApiSuccess(c, tag)
}

func DeleteQuotaPoolBudgetTag(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("tag_id"))
	if err != nil || id <= 0 {
		writeQuotaPoolError(c, model.ErrQuotaPoolBudgetTagNotFound)
		return
	}
	deleted, err := model.DeleteQuotaPoolBudgetTag(id)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, 0, "quota_pool.budget_tag.delete", map[string]any{"tag_id": id, "name": deleted.Name})
	common.ApiSuccess(c, nil)
}

func SetQuotaPoolBudgetTag(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	poolId, ok := parseQuotaPoolID(c)
	if !ok {
		return
	}
	var request quotaPoolBudgetTagAssignmentRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.TagId < 0 {
		writeQuotaPoolError(c, model.ErrQuotaPoolBudgetTagNotFound)
		return
	}
	previousTagId, err := model.SetQuotaPoolBudgetTag(poolId, request.TagId)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, poolId, "quota_pool.budget_tag.assign", map[string]any{
		"before_tag_id": previousTagId, "after_tag_id": request.TagId,
	})
	common.ApiSuccess(c, nil)
}

func ReplaceQuotaPoolBudgetTagPools(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	tagId, err := strconv.Atoi(c.Param("tag_id"))
	if err != nil || tagId <= 0 {
		writeQuotaPoolError(c, model.ErrQuotaPoolBudgetTagNotFound)
		return
	}
	var request quotaPoolBudgetTagPoolsRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		writeQuotaPoolError(c, model.ErrQuotaPoolNotFound)
		return
	}
	change, err := model.ReplaceQuotaPoolBudgetTagPools(tagId, request.PoolIds)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	recordQuotaPoolAudit(c, 0, "quota_pool.budget_tag.assign_pools", map[string]any{
		"tag_id": tagId, "pool_count": len(request.PoolIds), "before": change.Before, "after": change.After,
	})
	common.ApiSuccess(c, nil)
}

func GetQuotaPoolBudgetStats(c *gin.Context) {
	if !requireQuotaPoolFeature(c) {
		return
	}
	startMonth := c.Query("start_month")
	endMonth := c.Query("end_month")
	if startMonth == "" && endMonth == "" {
		currentMonth := time.Now().In(common.BeijingTimeLocation).Format("2006-01")
		startMonth, endMonth = currentMonth, currentMonth
	} else if startMonth == "" || endMonth == "" {
		writeQuotaPoolError(c, model.ErrQuotaPoolBudgetMonthRangeInvalid)
		return
	}
	stats, err := model.GetQuotaPoolBudgetStats(startMonth, endMonth, common.BeijingTimeLocation)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}
