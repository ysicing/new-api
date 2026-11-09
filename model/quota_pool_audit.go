package model

import (
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/QuantumNous/new-api/common"
)

type QuotaPoolOperationLog struct {
	Log
	EventId    string `json:"event_id,omitempty"`
	ActorRole  int    `json:"actor_role,omitempty"`
	SourceRank int    `json:"-"`
}

type QuotaPoolOperationLogView struct {
	ViewerRole int
	SelfView   bool
}

func quotaPoolAuditScope(metadata string, poolIds []int) (string, []any) {
	conditions := make([]string, 0, len(poolIds))
	args := make([]any, 0, len(poolIds)*2)
	for _, poolId := range poolIds {
		conditions = append(conditions, "("+metadata+" LIKE ? OR "+metadata+" LIKE ?)")
		args = append(args, fmt.Sprintf(`%%"quota_pool_id":%d}%%`, poolId), fmt.Sprintf(`%%"quota_pool_id":%d,%%`, poolId))
	}
	return "(" + strings.Join(conditions, " OR ") + ")", args
}

func quotaPoolAuditActionScope(metadata, action string) (string, []any, error) {
	encoded, err := common.Marshal(action)
	if err != nil {
		return "", nil, err
	}
	pattern := `"op":{"action":` + string(encoded)
	escape := " ESCAPE '!'"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		pattern = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(pattern)
		escape = ""
	} else {
		pattern = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(pattern)
	}
	return "(" + metadata + " LIKE ?" + escape + " OR " + metadata + " LIKE ?" + escape + ")", []any{"%" + pattern + ",%", "%" + pattern + "}%"}, nil
}

// ListQuotaPoolOperationLogs 合并历史 usage 日志和独立审计事件，在数据库分页前完成筛选。
// 未指定查看者时使用 self 投影；全局入口必须显式传入实际角色。
func ListQuotaPoolOperationLogs(poolId int, page *common.PageInfo, action string, views ...QuotaPoolOperationLogView) ([]QuotaPoolOperationLog, int64, error) {
	view := QuotaPoolOperationLogView{ViewerRole: common.RoleCommonUser, SelfView: true}
	if len(views) > 0 {
		view = views[0]
	}
	poolIds := []int{poolId}
	pool, err := GetQuotaPoolById(poolId)
	if err == nil && pool.LegacyDefault {
		// 存量池历史充值日志使用兼容成员标识 0，和真实池 ID 一并查询。
		poolIds = append(poolIds, 0)
	} else if err != nil && !IsQuotaPoolNotFound(err) {
		return nil, 0, err
	}
	logDB := LOG_DB
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		logDB = logDB.WithContext(clickhouse.Context(logDB.Statement.Context, clickhouse.WithSettings(clickhouse.Settings{
			"output_format_native_write_json_as_string": 1,
			"output_format_json_quote_64bit_integers":   0,
		})))
	}

	auditMetadata := "CAST(other AS TEXT)"
	if common.UsingLogDatabase(common.DatabaseTypeMySQL) {
		auditMetadata = "CAST(other AS CHAR)"
	} else if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		auditMetadata = "toJSONString(other)"
	}
	// MySQL 的 JSON 类型会规范化分隔符空格，查询兼容旧 TEXT 和新 JSON 存储。
	legacyMetadata := `REPLACE(REPLACE(other, ': ', ':'), ', ', ',')`
	normalizedAuditMetadata := "REPLACE(REPLACE(" + auditMetadata + ", ': ', ':'), ', ', ',')"
	poolScope, poolArgs := quotaPoolAuditScope(legacyMetadata, poolIds)
	legacy := logDB.Model(&Log{}).
		Where("type IN ?", []int{LogTypeManage, LogTypeTopup}).
		Where(poolScope, poolArgs...)
	auditScope, auditArgs := quotaPoolAuditScope(normalizedAuditMetadata, poolIds)
	audits := logDB.Model(&AuditLog{}).Where("category = ?", AuditCategoryOperation).Where(auditScope, auditArgs...)

	if action = strings.TrimSpace(action); action != "" {
		actionScope, actionArgs, err := quotaPoolAuditActionScope(legacyMetadata, action)
		if err != nil {
			return nil, 0, err
		}
		legacy = legacy.Where(actionScope, actionArgs...)
		audits = audits.Where("action = ?", action)
	}
	if view.ViewerRole < common.RoleRootUser {
		audits = audits.Where("actor_role IN ?", []int{common.RoleCommonUser, common.RoleQuotaPoolSuperAdmin, common.RoleAdminUser})
	}

	// 新充值同时写财务日志与审计事件；以审计为准，避免双计数，也防止根用户审计从财务副本绕过角色过滤。
	duplicateMetadata := "CAST(pool_audit_duplicates.other AS TEXT)"
	if common.UsingLogDatabase(common.DatabaseTypeMySQL) {
		duplicateMetadata = "CAST(pool_audit_duplicates.other AS CHAR)"
	} else if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		duplicateMetadata = "toJSONString(pool_audit_duplicates.other)"
	}
	duplicateScope, duplicateArgs := quotaPoolAuditScope("REPLACE(REPLACE("+duplicateMetadata+", ': ', ':'), ', ', ',')", poolIds)
	for _, rechargeAction := range []string{"quota_pool.member_recharge", "quota_pool.member_reclaim", "user.quota_pool_recharge"} {
		actionScope, actionArgs, err := quotaPoolAuditActionScope(legacyMetadata, rechargeAction)
		if err != nil {
			return nil, 0, err
		}
		duplicates := logDB.Table("audit_logs AS pool_audit_duplicates").
			Select("request_id").
			Where("pool_audit_duplicates.request_id <> ''").
			Where("pool_audit_duplicates.category = ? AND pool_audit_duplicates.action = ?", AuditCategoryOperation, rechargeAction).
			Where(duplicateScope, duplicateArgs...)
		args := append([]any{LogTypeTopup}, actionArgs...)
		args = append(args, duplicates)
		legacy = legacy.Where("NOT (type = ? AND "+actionScope+" AND request_id IN (?))", args...)
	}

	legacy = legacy.Select("id, user_id, username, created_at, type, content, ip, request_id, other, '' AS event_id, 0 AS actor_role, 0 AS source_rank")
	audits = audits.Select("id, user_id, username, created_at, ? AS type, content, ip, request_id, "+auditMetadata+" AS other, event_id, actor_role, 1 AS source_rank", LogTypeManage)
	query := logDB.Table("(? UNION ALL ?) AS pool_operations", legacy, audits)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	logs := make([]QuotaPoolOperationLog, 0)
	if err := query.Order("created_at DESC, source_rank DESC, id DESC, event_id DESC").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	visibility := logOtherVisibilityUser
	if !view.SelfView && view.ViewerRole >= common.RoleRootUser {
		visibility = logOtherVisibilityRoot
	} else if !view.SelfView && view.ViewerRole >= common.RoleAdminUser {
		visibility = logOtherVisibilityAdmin
	}
	for index := range logs {
		logs[index].Other = formatLogOtherJSON(logs[index].Other, visibility)
		if logs[index].SourceRank == 1 {
			logs[index].Id = -logs[index].Id
			var operation AuditOther
			if err := common.UnmarshalJsonStr(logs[index].Other, &operation); err != nil {
				return nil, 0, err
			}
			if operation.Op != nil && (operation.Op.Action == "quota_pool.member_recharge" || operation.Op.Action == "quota_pool.member_reclaim" || operation.Op.Action == "user.quota_pool_recharge") {
				logs[index].Type = LogTypeTopup
			}
		}
	}
	return logs, total, nil
}
