# 额度池操作日志可读性设计

## 目标

让新产生的额度池操作日志以自然、明确的句子展示操作人、操作对象、额度池和金额，避免直接显示 `quota_pool.member_move` 等内部 action，也避免只有额度池 ID 的含糊文案。

本次不迁移、不重写、不解析历史日志。历史记录继续显示原始 `content`。

## 根因

新日志通过 `RecordOperationAuditLog` 将稳定 action 和参数写入 `other.op`，同时把 action 作为 `content` 兜底。额度池日志页面目前只渲染 `content`，且 `QuotaPoolOperationLog` 前端类型没有暴露 `other`，因此新日志显示内部 action。

旧额度池日志把被操作成员写在 `Log.UserId/Username`，新审计日志则把实际操作人写在该字段中。由于本次不兼容历史数据，新的表头和渲染语义只以新结构化日志为准。

## 新日志数据

### 写入原则

新额度池审计日志继续保存语言无关的 `action + params`。在写日志时补充操作发生时的名称快照，不在展示时跨库查询：

- `operator_id`、`operator_name`：实际操作人；现有 `admin_info` 已提供。
- `user_id`、`user_name`：被操作成员。
- `quota_pool_id`、`quota_pool_name`：当前日志所属额度池。
- `target_pool_id`、`target_pool_name`：迁移或移出的目标额度池。
- `amount`：充值、回收或临时充值额度，保持内部额度单位。
- `fields`：更新配置时的字段数量。
- `enabled`：启用或停用后的状态。

名称是审计快照。用户或额度池之后改名、删除，不改变历史日志展示。

### 补全方式

`recordQuotaPoolAudit` 在写入前根据参数中存在的 ID 做最多一次用户查询和最多两次额度池查询：

- 有 `user_id` 时读取用户显示名，优先 `display_name`，其次 `username`，写入 `user_name`。
- `poolId > 0` 时读取当前额度池名称，写入 `quota_pool_name`；删除动作允许读取刚被软删除的额度池快照。
- 有 `target_pool_id` 且与当前池不同，读取目标池名称，写入 `target_pool_name`。
- 任一名称查询失败时仍写日志，只保留对应 ID，不能影响原业务成功响应。

额度池管理操作频率低，写入时的少量主库查询可接受；这避免日志读取时跨 `LOG_DB` 与主库关联，也避免前端逐行请求。

## Action 与展示模板

支持以下新日志 action：

- `quota_pool.create`：创建额度池“{{quotaPoolName}}”，初始额度 {{amount}}
- `quota_pool.sync_system`：同步系统额度池
- `quota_pool.update`：更新“{{quotaPoolName}}”的配置（{{fields}} 项）
- `quota_pool.enabled`：启用或停用额度池“{{quotaPoolName}}”
- `quota_pool.delete`：删除额度池“{{quotaPoolName}}”
- `quota_pool.refill`：为“{{quotaPoolName}}”临时充值 {{amount}}
- `quota_pool.self_update`：更新“{{quotaPoolName}}”的自动充值配置（{{fields}} 项）
- `quota_pool.member_add`：将成员 {{userName}}（ID: {{userId}}）加入“{{quotaPoolName}}”
- `quota_pool.member_move`：将成员 {{userName}}（ID: {{userId}}）迁入“{{quotaPoolName}}”
- `quota_pool.member_remove`：将成员 {{userName}}（ID: {{userId}}）移出至“{{targetPoolName}}”，回收额度 {{amount}}；新日志统一使用 `amount` 参数，不再新增 `reclaimed_amount`
- `quota_pool.member_recharge`：为成员 {{userName}}（ID: {{userId}}）充值 {{amount}}
- `quota_pool.member_reclaim`：从成员 {{userName}}（ID: {{userId}}）回收 {{amount}}
- `quota_pool.admin_grant`：将成员 {{userName}}（ID: {{userId}}）设为池管理员
- `quota_pool.admin_revoke`：撤销成员 {{userName}}（ID: {{userId}}）的池管理员权限

名称缺失时使用 `#ID`，金额通过现有 `formatQuota` 渲染。布尔状态和字段数量在前端转换为本地化文本。

## 前端展示

额度池操作日志使用三列：

- `操作人`：新日志的 `username`，为空时显示 `#user_id`。
- `操作详情`：解析 `other.op`，通过额度池 action 模板和 i18n 渲染自然语言。
- `时间`：沿用现有时间格式。

前端 `QuotaPoolOperationLog` 增加 `other: string`。新增额度池专用纯函数：

1. 安全解析 `other`；无效 JSON 返回原始 `content`。
2. 仅识别 `quota_pool.*` action；未知 action 返回原始 `content`。
3. 将内部额度单位转成展示额度。
4. 使用快照名称，缺失时回退 ID。

所有新模板通过 `i18n-translate` 脚本化补齐 `en`、`zh`、`zh-TW`、`fr`、`ru`、`ja`、`vi`。

## API 与数据库

- 不新增数据库表或列。
- 不改变 `/api/quota_pool/:id/operation_logs` 和 `/api/quota_pool/self/operation_logs` 路径、分页及权限。
- API 继续返回日志记录，仅由前端读取现有 `other` 字段。
- 不在读取接口中跨库 JOIN 或批量补全名称。

## 测试与验收

### 后端

- 成员操作写入 `user_id/user_name/quota_pool_id/quota_pool_name`。
- 成员移出额外写入 `target_pool_id/target_pool_name/amount`。
- 充值、回收和临时充值保存正确内部额度值。
- 名称查询失败不影响日志写入或原业务响应。
- 写入的是结构化 action，不存固定中文句子。

### 前端

- 每个额度池 action 输出自然语言。
- 金额使用 `formatQuota`。
- 用户名、池名缺失时回退 ID。
- 无效 JSON、未知 action 和无 `op` 时回退原始 `content`。
- 表头显示“操作人、操作详情、时间”。
- 七语言同步报告无缺失、额外或未翻译条目。

### 最终验证

- 后端额度池 Controller/Model 测试与 `go test ./...`。
- 前端额度池测试、全量测试、typecheck、涉及文件 lint、i18n 同步和生产构建。
- `git diff --check`，确认无构建产物进入提交。

## 非目标

- 不转换或回填已有数据库日志。
- 不为历史中文文案编写正则解析规则。
- 不增加日志筛选、导出或详情弹窗。
- 不改变额度池操作权限和资金行为。
