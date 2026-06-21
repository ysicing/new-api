package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
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

func quotaPoolPageInfo(c *gin.Context) *common.PageInfo {
	page, _ := strconv.Atoi(c.Query("p"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if page <= 0 {
		page = 1
	}
	return &common.PageInfo{Page: page, PageSize: pageSize}
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
		updates["name"] = *req.Name
	}
	if req.AutoRechargeAmount != nil {
		if *req.AutoRechargeAmount < 0 {
			updates["auto_recharge_amount"] = model.QuotaPoolAutoRechargeInherit
		} else {
			updates["auto_recharge_amount"] = quotaAmountToInternal(*req.AutoRechargeAmount)
		}
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
	if req.MonthlyRefillAmount != nil {
		if *req.MonthlyRefillAmount < 0 {
			common.ApiError(c, errors.New("月度扩容金额不能小于 0"))
			return
		}
		updates["monthly_refill_amount"] = quotaAmountToInternal(*req.MonthlyRefillAmount)
	}
	if req.MonthlyRefillDay != nil {
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
	if err := model.UpdateQuotaPoolConfig(id, updates); err != nil {
		common.ApiError(c, err)
		return
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
	if err := model.DeleteQuotaPool(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RefillQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
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
	items, total, err := model.ListQuotaPoolTransactions(id, quotaPoolPageInfo(c))
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

func addUserToQuotaPool(c *gin.Context, poolId int, userId int, initialRecharge bool) {
	user := &model.User{}
	if err := model.DB.First(user, "id = ?", userId).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Role != common.RoleCommonUser {
		common.ApiError(c, errors.New("只能添加普通用户到额度池"))
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
	addUserToQuotaPool(c, id, req.UserId, true)
}

func MoveUserQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
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
	addUserToQuotaPool(c, req.PoolId, userId, req.PoolId != model.QuotaPoolDefaultUserPoolId)
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

func RechargeQuotaPoolMember(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
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

func GrantQuotaPoolAdmin(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
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
	if req.Level != model.QuotaPoolAdminLevelV1 && req.Level != model.QuotaPoolAdminLevelV2 {
		common.ApiError(c, errors.New("额度池管理员等级无效"))
		return
	}
	user := &model.User{}
	if err := model.DB.First(user, "id = ?", req.UserId).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Role != common.RoleCommonUser {
		common.ApiError(c, errors.New("只能任命普通用户为额度池管理员"))
		return
	}
	if user.QuotaPoolId == model.QuotaPoolDefaultUserPoolId {
		if _, err := moveUserToQuotaPoolForController(c, id, req.UserId, true); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if err := model.GrantQuotaPoolAdmin(id, req.UserId, req.Level); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RevokeQuotaPoolAdmin(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
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
	common.ApiSuccess(c, nil)
}

func GetSelfQuotaPool(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	admin, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV1)
	if !ok {
		return
	}
	pool, err := model.GetQuotaPoolListItemById(admin.PoolId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"pool": pool, "admin": admin})
}

func GetSelfQuotaPoolTransactions(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	admin, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV1)
	if !ok {
		return
	}
	items, total, err := model.ListQuotaPoolTransactions(admin.PoolId, quotaPoolPageInfo(c))
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
	if user.QuotaPoolId != model.QuotaPoolDefaultUserPoolId {
		common.ApiError(c, errors.New("池管理员只能添加默认池用户"))
		return
	}
	addUserToQuotaPool(c, admin.PoolId, req.UserId, true)
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

func GrantSelfQuotaPoolAdmin(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	admin, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV2)
	if !ok {
		return
	}
	var req quotaPoolAdminRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Level != model.QuotaPoolAdminLevelV1 {
		common.ApiError(c, errors.New("池 v2 只能任命 v1 管理员"))
		return
	}
	user := &model.User{}
	if err := model.DB.First(user, "id = ?", req.UserId).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Role != common.RoleCommonUser {
		common.ApiError(c, errors.New("只能任命普通用户为额度池管理员"))
		return
	}
	if user.QuotaPoolId == model.QuotaPoolDefaultUserPoolId {
		if _, err := moveUserToQuotaPoolForController(c, admin.PoolId, req.UserId, true); err != nil {
			common.ApiError(c, err)
			return
		}
	} else if user.QuotaPoolId != admin.PoolId {
		common.ApiError(c, errors.New("池 v2 只能任命默认池或本池用户"))
		return
	}
	if err := model.GrantQuotaPoolAdmin(admin.PoolId, req.UserId, model.QuotaPoolAdminLevelV1); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RevokeSelfQuotaPoolAdmin(c *gin.Context) {
	if !requireQuotaPoolEnabled(c) {
		return
	}
	admin, ok := requireQuotaPoolAdmin(c, model.QuotaPoolAdminLevelV2)
	if !ok {
		return
	}
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.ApiError(c, errors.New("用户 ID 无效"))
		return
	}
	targetAdmin := &model.QuotaPoolAdmin{}
	if err := model.DB.First(targetAdmin, "pool_id = ? AND user_id = ?", admin.PoolId, userId).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if targetAdmin.Level != model.QuotaPoolAdminLevelV1 {
		common.ApiError(c, errors.New("池 v2 只能移除 v1 管理员"))
		return
	}
	if err := model.RevokeQuotaPoolAdmin(admin.PoolId, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
