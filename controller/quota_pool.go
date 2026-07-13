package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type quotaPoolCreateRequest struct {
	Name      string  `json:"name"`
	BaseQuota float64 `json:"base_quota"`
}

type quotaPoolUpdateRequest struct {
	Name                 *string  `json:"name"`
	BaseQuota            *float64 `json:"base_quota"`
	AutoRechargeAmount   *float64 `json:"auto_recharge_amount"`
	WeeklyLimit          *int     `json:"weekly_limit"`
	MonthlyLimit         *int     `json:"monthly_limit"`
	MonthlyRefillEnabled *bool    `json:"monthly_refill_enabled"`
	MonthlyRefillAmount  *float64 `json:"monthly_refill_amount"`
	MonthlyRefillDay     *int     `json:"monthly_refill_day"`
}

type quotaPoolRefillRequest struct {
	Amount float64 `json:"amount"`
}

type quotaPoolMemberRequest struct {
	UserId int `json:"user_id"`
}

type quotaPoolMoveRequest struct {
	PoolId int `json:"pool_id"`
}

type quotaPoolAdminRequest struct {
	UserId int `json:"user_id"`
	Level  int `json:"level"`
}

func quotaAmountToInternal(amount float64) int {
	return int(amount * common.QuotaPerUnit)
}

func quotaPoolAutoRechargeAmountUpdate(amount float64) (int, error) {
	if amount < 0 {
		return model.QuotaPoolAutoRechargeInherit, nil
	}
	amountQuota := quotaAmountToInternal(amount)
	systemAmountQuota := quotaAmountToInternal(float64(operation_setting.GetAutoRechargeSetting().Amount))
	if amountQuota > 0 && systemAmountQuota > 0 && amountQuota < systemAmountQuota {
		return 0, errors.New("自动充值额度小于系统默认充值额度")
	}
	return amountQuota, nil
}

func quotaPoolPageInfo(c *gin.Context) *common.PageInfo {
	return common.GetPageQuery(c)
}

func quotaPoolTransactionFilter(c *gin.Context) *model.QuotaPoolTransactionFilter {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	return &model.QuotaPoolTransactionFilter{
		UserKeyword:    strings.TrimSpace(c.Query("user")),
		Types:          quotaPoolTransactionTypes(c.Query("transaction_type")),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
	}
}

func quotaPoolTransactionTypes(transactionType string) []string {
	switch strings.TrimSpace(transactionType) {
	case "":
		return nil
	case "manual":
		return []string{
			model.QuotaPoolTransactionInitialFund,
			model.QuotaPoolTransactionManualRefill,
			model.QuotaPoolTransactionAllocateManual,
			model.QuotaPoolTransactionAdjustBase,
		}
	case "auto":
		return []string{
			model.QuotaPoolTransactionMonthlyRefill,
			model.QuotaPoolTransactionAllocateAuto,
		}
	case "reclaim":
		return []string{model.QuotaPoolTransactionReclaimUser}
	default:
		return []string{transactionType}
	}
}

func quotaPoolStatsRange(c *gin.Context) (int64, int64) {
	now := time.Now()
	period := c.DefaultQuery("period", "week")
	var start time.Time
	if period == "month" {
		start = now.AddDate(0, -1, 0)
	} else {
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		period = "week"
	}
	return start.Unix(), now.Unix()
}

func quotaPoolAdminLevelName(level int) string {
	if level == model.QuotaPoolAdminLevelV1 {
		return "池管理员 v1"
	}
	return "成员"
}

func quotaPoolAdminLogInfo(c *gin.Context, poolId int) map[string]interface{} {
	return map[string]interface{}{
		"admin_id":       c.GetInt("id"),
		"admin_username": c.GetString("username"),
		"quota_pool_id":  poolId,
	}
}

func recordQuotaPoolMemberManageLog(c *gin.Context, poolId int, userId int, content string) {
	model.RecordLogWithAdminInfo(userId, model.LogTypeManage, content, quotaPoolAdminLogInfo(c, poolId))
}

func recordQuotaPoolManageLog(c *gin.Context, poolId int, content string) {
	model.RecordLogWithAdminInfo(c.GetInt("id"), model.LogTypeManage, content, quotaPoolAdminLogInfo(c, poolId))
}

func quotaPoolRechargeAmountLabel(amount int) string {
	if amount == model.QuotaPoolAutoRechargeInherit {
		return "继承全局配置"
	}
	if amount == 0 {
		return "关闭"
	}
	return logger.LogQuota(amount)
}

func quotaPoolLimitLabel(limit int) string {
	if limit == model.QuotaPoolAutoRechargeInherit {
		return "继承全局配置"
	}
	if limit == 0 {
		return "不限制"
	}
	return fmt.Sprintf("%d 次", limit)
}

func quotaPoolEnabledLabel(enabled bool) string {
	if enabled {
		return "启用"
	}
	return "关闭"
}

func quotaPoolConfigChangeDescriptions(pool *model.QuotaPool, updates map[string]interface{}) []string {
	if pool == nil || len(updates) == 0 {
		return nil
	}
	descriptions := make([]string, 0, len(updates))
	if name, ok := updates["name"].(string); ok && name != pool.Name {
		descriptions = append(descriptions, fmt.Sprintf("名称 %s -> %s", pool.Name, name))
	}
	if amount, ok := updates["auto_recharge_amount"].(int); ok && amount != pool.AutoRechargeAmount {
		descriptions = append(descriptions, fmt.Sprintf("充值金额 %s -> %s", quotaPoolRechargeAmountLabel(pool.AutoRechargeAmount), quotaPoolRechargeAmountLabel(amount)))
	}
	if limit, ok := updates["weekly_limit"].(int); ok && limit != pool.WeeklyLimit {
		descriptions = append(descriptions, fmt.Sprintf("周自动充值次数 %s -> %s", quotaPoolLimitLabel(pool.WeeklyLimit), quotaPoolLimitLabel(limit)))
	}
	if limit, ok := updates["monthly_limit"].(int); ok && limit != pool.MonthlyLimit {
		descriptions = append(descriptions, fmt.Sprintf("月自动充值次数 %s -> %s", quotaPoolLimitLabel(pool.MonthlyLimit), quotaPoolLimitLabel(limit)))
	}
	if enabled, ok := updates["monthly_refill_enabled"].(bool); ok && enabled != pool.MonthlyRefillEnabled {
		descriptions = append(descriptions, fmt.Sprintf("月度扩容 %s -> %s", quotaPoolEnabledLabel(pool.MonthlyRefillEnabled), quotaPoolEnabledLabel(enabled)))
	}
	if amount, ok := updates["monthly_refill_amount"].(int); ok && amount != pool.MonthlyRefillAmount {
		descriptions = append(descriptions, fmt.Sprintf("月扩容金额 %s -> %s", logger.LogQuota(pool.MonthlyRefillAmount), logger.LogQuota(amount)))
	}
	if day, ok := updates["monthly_refill_day"].(int); ok && day != pool.MonthlyRefillDay {
		descriptions = append(descriptions, fmt.Sprintf("扩容日期 %d -> %d", pool.MonthlyRefillDay, day))
	}
	return descriptions
}

func parseQuotaPoolId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 0 {
		common.ApiError(c, errors.New("额度池 ID 无效"))
		return 0, false
	}
	return id, true
}

func requireQuotaPoolEnabled(c *gin.Context) bool {
	if !common.QuotaPoolEnabled {
		common.ApiError(c, errors.New("额度池功能未启用"))
		return false
	}
	return true
}

func currentQuotaPoolAdmin(c *gin.Context) (*model.QuotaPoolAdminSummary, error) {
	userId := c.GetInt("id")
	return model.GetQuotaPoolAdminSummary(userId)
}

func requireQuotaPoolAdmin(c *gin.Context, minLevel int) (*model.QuotaPoolAdminSummary, bool) {
	summary, err := currentQuotaPoolAdmin(c)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	if summary == nil || summary.Level < minLevel {
		common.ApiError(c, errors.New("无额度池权限"))
		return nil, false
	}
	return summary, true
}

func isSystemAdmin(c *gin.Context) bool {
	return c.GetInt("role") >= common.RoleAdminUser
}

func isQuotaPoolSuperAdmin(c *gin.Context) bool {
	return c.GetInt("role") == common.RoleQuotaPoolSuperAdmin
}

func requireSystemAdmin(c *gin.Context) bool {
	if !isSystemAdmin(c) {
		common.ApiError(c, errors.New("无权限"))
		return false
	}
	return true
}

func requireQuotaPoolAdminManager(c *gin.Context) bool {
	if isSystemAdmin(c) || isQuotaPoolSuperAdmin(c) {
		return true
	}
	common.ApiError(c, errors.New("无额度池权限"))
	return false
}

func isSystemRoot(c *gin.Context) bool {
	return c.GetInt("role") == common.RoleRootUser
}

func GetQuotaPools(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	items, err := model.ListQuotaPools()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func CreateQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	var req quotaPoolCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	pool, err := model.CreateQuotaPool(req.Name, quotaAmountToInternal(req.BaseQuota), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	adminId := c.GetInt("id")
	adminInfo := map[string]interface{}{
		"admin_id":       adminId,
		"admin_username": c.GetString("username"),
		"quota_pool_id":  pool.Id,
	}
	model.RecordLogWithAdminInfo(adminId, model.LogTypeManage, fmt.Sprintf("创建额度池 %s，初始额度 %s", pool.Name, logger.LogQuota(pool.BaseQuota)), adminInfo)
	common.ApiSuccess(c, pool)
}

func SyncDefaultQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	if err := model.SyncSystemQuotaPools(); err != nil {
		common.ApiError(c, err)
		return
	}
	pool, err := model.SyncDefaultQuotaPool()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordQuotaPoolManageLog(c, pool.Id, fmt.Sprintf("同步默认额度池(ID:%d)", pool.Id))
	common.ApiSuccess(c, pool)
}

func GetQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	pool, err := model.GetQuotaPoolById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, pool)
}

func UpdateQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	root := isSystemRoot(c)
	if !root && !requireQuotaPoolAdminManager(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	var req quotaPoolUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	updates := map[string]interface{}{}
	if req.Name != nil {
		if !root {
			common.ApiError(c, errors.New("无权限调整额度池名称"))
			return
		}
		updates["name"] = *req.Name
	}
	if req.BaseQuota != nil {
		if !root {
			common.ApiError(c, errors.New("无权限调整额度池总额度"))
			return
		}
		if *req.BaseQuota <= 0 {
			common.ApiError(c, errors.New("额度池总额度必须大于 0"))
			return
		}
		updates["base_quota"] = quotaAmountToInternal(*req.BaseQuota)
	}
	if req.AutoRechargeAmount != nil {
		amount, err := quotaPoolAutoRechargeAmountUpdate(*req.AutoRechargeAmount)
		if err != nil {
			common.ApiError(c, err)
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
	if req.MonthlyRefillEnabled != nil {
		if !root {
			common.ApiError(c, errors.New("无权限调整月度扩容"))
			return
		}
		updates["monthly_refill_enabled"] = *req.MonthlyRefillEnabled
	}
	if req.MonthlyRefillAmount != nil {
		if !root {
			common.ApiError(c, errors.New("无权限调整月度扩容"))
			return
		}
		if *req.MonthlyRefillAmount < 0 {
			common.ApiError(c, errors.New("月度扩容金额不能小于 0"))
			return
		}
		updates["monthly_refill_amount"] = quotaAmountToInternal(*req.MonthlyRefillAmount)
	}
	if req.MonthlyRefillDay != nil {
		if !root {
			common.ApiError(c, errors.New("无权限调整月度扩容"))
			return
		}
		if *req.MonthlyRefillDay < 1 || *req.MonthlyRefillDay > 28 {
			common.ApiError(c, errors.New("月度扩容日期必须在 1 到 28 之间"))
			return
		}
		updates["monthly_refill_day"] = *req.MonthlyRefillDay
	}
	if req.MonthlyRefillEnabled != nil || req.MonthlyRefillAmount != nil {
		pool, err := model.GetQuotaPoolById(id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		effectiveEnabled := pool.MonthlyRefillEnabled
		if req.MonthlyRefillEnabled != nil {
			effectiveEnabled = *req.MonthlyRefillEnabled
		}
		effectiveAmount := pool.MonthlyRefillAmount
		if req.MonthlyRefillAmount != nil {
			effectiveAmount = quotaAmountToInternal(*req.MonthlyRefillAmount)
		}
		if effectiveEnabled && effectiveAmount <= 0 {
			common.ApiError(c, errors.New("启用月度扩容时扩容金额必须大于 0"))
			return
		}
	}
	var beforePool *model.QuotaPool
	if len(updates) > 0 {
		pool, err := model.GetQuotaPoolById(id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		beforePool = pool
	}
	change, err := model.UpdateQuotaPoolConfig(id, updates, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if descriptions := quotaPoolConfigChangeDescriptions(beforePool, updates); len(descriptions) > 0 {
		recordQuotaPoolManageLog(c, id, fmt.Sprintf("修改额度池(ID:%d)配置：%s", id, strings.Join(descriptions, "；")))
	}
	if change != nil && change.Amount != 0 {
		recordQuotaPoolManageLog(c, id, fmt.Sprintf("调整额度池(ID:%d)总额度，额度池余额从 %s 变为 %s", id, logger.LogQuota(change.QuotaBefore), logger.LogQuota(change.QuotaAfter)))
	}
	common.ApiSuccess(c, nil)
}

func UpdateSelfQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	admin, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV1)
	if !ok {
		return
	}
	var req quotaPoolUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	updates := map[string]interface{}{}
	if req.AutoRechargeAmount != nil {
		amount, err := quotaPoolAutoRechargeAmountUpdate(*req.AutoRechargeAmount)
		if err != nil {
			common.ApiError(c, err)
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
	var beforePool *model.QuotaPool
	if len(updates) > 0 {
		pool, err := model.GetQuotaPoolById(admin.PoolId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		beforePool = pool
	}
	if _, err := model.UpdateQuotaPoolConfig(admin.PoolId, updates, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	if descriptions := quotaPoolConfigChangeDescriptions(beforePool, updates); len(descriptions) > 0 {
		recordQuotaPoolManageLog(c, admin.PoolId, fmt.Sprintf("修改额度池(ID:%d)配置：%s", admin.PoolId, strings.Join(descriptions, "；")))
	}
	common.ApiSuccess(c, nil)
}

func EnableQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	if err := model.SetQuotaPoolEnabled(id, true); err != nil {
		common.ApiError(c, err)
		return
	}
	recordQuotaPoolManageLog(c, id, fmt.Sprintf("启用额度池(ID:%d)", id))
	common.ApiSuccess(c, nil)
}

func DisableQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	if err := model.SetQuotaPoolEnabled(id, false); err != nil {
		common.ApiError(c, err)
		return
	}
	recordQuotaPoolManageLog(c, id, fmt.Sprintf("禁用额度池(ID:%d)", id))
	common.ApiSuccess(c, nil)
}

func DeleteQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	pool, err := model.GetQuotaPoolById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteQuotaPool(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordQuotaPoolManageLog(c, id, fmt.Sprintf("删除额度池(ID:%d) %s", id, pool.Name))
	common.ApiSuccess(c, nil)
}

func RefillQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	if !requireSystemAdmin(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	var req quotaPoolRefillRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	change, err := model.AddQuotaPoolManualRefill(id, quotaAmountToInternal(req.Amount), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordQuotaPoolManageLog(c, id, fmt.Sprintf("充值额度池(ID:%d)，额度池余额从 %s 变为 %s", id, logger.LogQuota(change.QuotaBefore), logger.LogQuota(change.QuotaAfter)))
	common.ApiSuccess(c, change)
}

func GetQuotaPoolTransactions(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	items, total, err := model.ListQuotaPoolTransactions(id, quotaPoolPageInfo(c), quotaPoolTransactionFilter(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": total})
}

func GetQuotaPoolMembers(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	items, total, err := model.ListQuotaPoolMembers(id, quotaPoolPageInfo(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": total})
}

func GetQuotaPoolStats(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	startTimestamp, endTimestamp := quotaPoolStatsRange(c)
	stats, err := model.GetQuotaPoolStats(id, startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func GetQuotaPoolCandidates(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	items, total, err := model.ListDefaultQuotaPoolCandidates(c.Query("keyword"), quotaPoolPageInfo(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": total})
}

func addUserToQuotaPool(c *gin.Context, poolId int, userId int, initialRecharge bool, logAction string) {
	user := &model.User{}
	if err := model.DB.First(user, "id = ?", userId).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if !model.IsQuotaPoolMemberRole(user.Role) {
		common.ApiError(c, errors.New("只能添加普通用户、池超级管理员或系统子管理员到额度池"))
		return
	}
	if user.Status != common.UserStatusEnabled {
		common.ApiError(c, errors.New("只能添加启用状态的用户到额度池"))
		return
	}
	if user.QuotaPoolId == poolId {
		common.ApiError(c, model.ErrQuotaPoolSamePool)
		return
	}
	result, err := model.MoveUserQuotaPool(userId, poolId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if result.Reclaimed {
		model.RecordQuotaPoolTransaction(result.Change.PoolId, model.QuotaPoolTransactionReclaimUser, result.Change.Amount, result.Change.QuotaBefore, result.Change.QuotaAfter, userId, c.GetInt("id"))
	}
	recordQuotaPoolMemberManageLog(c, poolId, userId, fmt.Sprintf("%s额度池(ID:%d)", logAction, poolId))
	if user.Role == common.RoleAdminUser && poolId != model.QuotaPoolDefaultUserPoolId {
		if err := model.GrantQuotaPoolAdmin(poolId, userId, model.QuotaPoolAdminLevelV1); err != nil {
			common.ApiError(c, err)
			return
		}
		recordQuotaPoolMemberManageLog(c, poolId, userId, fmt.Sprintf("任命用户为%s", quotaPoolAdminLevelName(model.QuotaPoolAdminLevelV1)))
	}
	warning := ""
	if initialRecharge && poolId != model.QuotaPoolDefaultUserPoolId {
		result := service.TryAutoRechargeUserById(userId)
		if !result.Recharged {
			warning = result.Reason
		}
	}
	if warning != "" {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "成员已添加，自动充值未完成：" + warning, "data": gin.H{"member_added": true, "initial_recharge_success": false}})
		return
	}
	common.ApiSuccess(c, gin.H{"member_added": true, "initial_recharge_success": initialRecharge && poolId != model.QuotaPoolDefaultUserPoolId})
}

func moveUserToQuotaPoolForController(c *gin.Context, poolId int, userId int, initialRecharge bool) (string, error) {
	result, err := model.MoveUserQuotaPool(userId, poolId)
	if err != nil {
		return "", err
	}
	if result.Reclaimed {
		model.RecordQuotaPoolTransaction(result.Change.PoolId, model.QuotaPoolTransactionReclaimUser, result.Change.Amount, result.Change.QuotaBefore, result.Change.QuotaAfter, userId, c.GetInt("id"))
	}
	recordQuotaPoolMemberManageLog(c, poolId, userId, fmt.Sprintf("将用户迁移到额度池(ID:%d)", poolId))
	if initialRecharge && poolId != model.QuotaPoolDefaultUserPoolId {
		result := service.TryAutoRechargeUserById(userId)
		if !result.Recharged {
			return result.Reason, nil
		}
	}
	return "", nil
}

func AddQuotaPoolMember(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	if !requireQuotaPoolAdminManager(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	if id == model.QuotaPoolDefaultUserPoolId {
		common.ApiError(c, model.ErrQuotaPoolDefaultReadonly)
		return
	}
	var req quotaPoolMemberRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	addUserToQuotaPool(c, id, req.UserId, true, "将用户加入")
}

func MoveUserQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	if !requireQuotaPoolAdminManager(c) {
		return
	}
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.ApiError(c, errors.New("用户 ID 无效"))
		return
	}
	var req quotaPoolMoveRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	addUserToQuotaPool(c, req.PoolId, userId, req.PoolId != model.QuotaPoolDefaultUserPoolId, "将用户迁移到")
}

func quotaPoolRechargeAmount(poolId int) (int, error) {
	pool, err := model.GetQuotaPoolById(poolId)
	if err != nil {
		return 0, err
	}
	cfg := operation_setting.GetAutoRechargeSetting()
	if pool == nil || pool.IsDefault || pool.AutoRechargeAmount == model.QuotaPoolAutoRechargeInherit {
		if cfg.Amount <= 0 {
			return 0, errors.New("自动充值金额未配置或小于等于0")
		}
		return int(float64(cfg.Amount) * common.QuotaPerUnit), nil
	}
	if pool.AutoRechargeAmount <= 0 {
		return 0, errors.New("该额度池未启用充值金额")
	}
	return pool.AutoRechargeAmount, nil
}

func quotaPoolAutoRechargeThreshold() int {
	cfg := operation_setting.GetAutoRechargeSetting()
	if cfg == nil || !cfg.Enabled || cfg.Threshold < 0 {
		return -1
	}
	return int(float64(cfg.Threshold) * common.QuotaPerUnit)
}

func rechargeQuotaPoolMember(c *gin.Context, poolId int, userId int) error {
	if poolId == model.QuotaPoolDefaultUserPoolId {
		return model.ErrQuotaPoolDefaultReadonly
	}
	amount, err := quotaPoolRechargeAmount(poolId)
	if err != nil {
		return err
	}
	transfer, err := model.TransferQuotaFromPoolToUser(poolId, userId, amount)
	if err != nil {
		return err
	}
	content := formatAdminTempQuotaLog(c.GetInt("id"), amount)
	adminInfo := map[string]interface{}{
		"admin_id":       c.GetInt("id"),
		"admin_username": c.GetString("username"),
		"quota_pool_id":  poolId,
	}
	model.RecordLogWithAdminInfo(userId, model.LogTypeManage, content, adminInfo)
	if transfer != nil && transfer.PoolChanged {
		model.RecordQuotaPoolTransaction(poolId, model.QuotaPoolTransactionAllocateManual, transfer.Change.Amount, transfer.Change.QuotaBefore, transfer.Change.QuotaAfter, userId, c.GetInt("id"))
	}
	return nil
}

func reclaimQuotaPoolMember(c *gin.Context, poolId int, userId int) error {
	if poolId == model.QuotaPoolDefaultUserPoolId {
		return model.ErrQuotaPoolDefaultReadonly
	}
	amount, err := quotaPoolRechargeAmount(poolId)
	if err != nil {
		return err
	}
	transfer, err := model.ReclaimQuotaFromUserToPool(poolId, userId, amount, quotaPoolAutoRechargeThreshold())
	if err != nil {
		return err
	}
	content := fmt.Sprintf("额度池管理员(ID:%d)减少%s临时额度", c.GetInt("id"), logger.LogQuota(amount))
	recordQuotaPoolMemberManageLog(c, poolId, userId, content)
	if transfer != nil && transfer.PoolChanged {
		model.RecordQuotaPoolTransaction(poolId, model.QuotaPoolTransactionReclaimUser, transfer.Change.Amount, transfer.Change.QuotaBefore, transfer.Change.QuotaAfter, userId, c.GetInt("id"))
	}
	return nil
}

func RechargeQuotaPoolMember(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	if !requireQuotaPoolAdminManager(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.ApiError(c, errors.New("用户 ID 无效"))
		return
	}
	if err := rechargeQuotaPoolMember(c, id, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ReclaimQuotaPoolMember(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	if !requireQuotaPoolAdminManager(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.ApiError(c, errors.New("用户 ID 无效"))
		return
	}
	if err := reclaimQuotaPoolMember(c, id, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func GrantQuotaPoolAdmin(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	if !requireQuotaPoolAdminManager(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	var req quotaPoolAdminRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Level != model.QuotaPoolAdminLevelV1 {
		common.ApiError(c, errors.New("额度池只支持池管理员权限"))
		return
	}
	user := &model.User{}
	if err := model.DB.First(user, "id = ?", req.UserId).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if !model.IsQuotaPoolMemberRole(user.Role) {
		common.ApiError(c, errors.New("只能任命普通用户、池超级管理员或系统子管理员为额度池管理员"))
		return
	}
	if model.IsQuotaPoolCandidateSourcePoolId(user.QuotaPoolId) {
		if _, err := moveUserToQuotaPoolForController(c, id, req.UserId, true); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if err := model.GrantQuotaPoolAdmin(id, req.UserId, req.Level); err != nil {
		common.ApiError(c, err)
		return
	}
	recordQuotaPoolMemberManageLog(c, id, req.UserId, fmt.Sprintf("任命用户为%s", quotaPoolAdminLevelName(req.Level)))
	common.ApiSuccess(c, nil)
}

func RevokeQuotaPoolAdmin(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	if !requireQuotaPoolAdminManager(c) {
		return
	}
	id, ok := parseQuotaPoolId(c)
	if !ok {
		return
	}
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.ApiError(c, errors.New("用户 ID 无效"))
		return
	}
	if err := model.RevokeQuotaPoolAdmin(id, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	recordQuotaPoolMemberManageLog(c, id, userId, "撤销额度池管理员权限")
	common.ApiSuccess(c, nil)
}

func GetSelfQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	admin, err := currentQuotaPoolAdmin(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	poolId := user.QuotaPoolId
	if admin != nil {
		poolId = admin.PoolId
	}
	if admin == nil && poolId == model.QuotaPoolDefaultUserPoolId {
		common.ApiSuccess(c, gin.H{"pool": nil, "admin": nil})
		return
	}
	pool, err := model.GetQuotaPoolListItemById(poolId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	adminContacts, err := model.ListQuotaPoolAdminContacts(poolId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"pool": pool, "admin": admin, "admin_contacts": adminContacts})
}

func GetSelfQuotaPoolTransactions(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	admin, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV1)
	if !ok {
		return
	}
	items, total, err := model.ListQuotaPoolTransactions(admin.PoolId, quotaPoolPageInfo(c), quotaPoolTransactionFilter(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": total})
}

func GetSelfQuotaPoolMembers(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	admin, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV1)
	if !ok {
		return
	}
	items, total, err := model.ListQuotaPoolMembers(admin.PoolId, quotaPoolPageInfo(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": total})
}

func GetSelfQuotaPoolStats(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	admin, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV1)
	if !ok {
		return
	}
	startTimestamp, endTimestamp := quotaPoolStatsRange(c)
	stats, err := model.GetQuotaPoolStats(admin.PoolId, startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func GetSelfQuotaPoolCandidates(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	if _, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV1); !ok {
		return
	}
	GetQuotaPoolCandidates(c)
}

func AddSelfQuotaPoolMember(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	admin, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV1)
	if !ok {
		return
	}
	var req quotaPoolMemberRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	user := &model.User{}
	if err := model.DB.First(user, "id = ?", req.UserId).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if !model.IsQuotaPoolCandidateSourcePoolId(user.QuotaPoolId) {
		common.ApiError(c, errors.New("池管理员只能添加默认池或新用户池用户"))
		return
	}
	addUserToQuotaPool(c, admin.PoolId, req.UserId, true, "将用户加入")
}

func RechargeSelfQuotaPoolMember(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	admin, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV1)
	if !ok {
		return
	}
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.ApiError(c, errors.New("用户 ID 无效"))
		return
	}
	if err := rechargeQuotaPoolMember(c, admin.PoolId, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ReclaimSelfQuotaPoolMember(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	admin, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV1)
	if !ok {
		return
	}
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.ApiError(c, errors.New("用户 ID 无效"))
		return
	}
	if err := reclaimQuotaPoolMember(c, admin.PoolId, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func GrantSelfQuotaPoolAdmin(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	common.ApiError(c, errors.New("池管理员不能任命或撤销池管理员"))
}

func RevokeSelfQuotaPoolAdmin(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	common.ApiError(c, errors.New("池管理员不能任命或撤销池管理员"))
}
