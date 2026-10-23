# 开发交接注意事项

## 额度池与 Root 账号

- `IsQuotaPoolMemberRole` 同时用于成员添加和池管理员任命，不能仅为放开 Root 迁移就把 Root 加入该角色集合。Root 迁移通过 `MoveUserBetweenQuotaPools` 单独开放，并在事务内核实操作人的 Root 身份。
- 迁移只改变额度和池归属，保留系统角色；普通用户、管理员及 Root 均可迁入禁用池。禁用池仍禁止从池内发放额度（含 Root 手动充值和自动充值），迁出时照常回收余额并记录流水。Root 直接维护用户余额仍沿用源分支的独立权限规则。前端目标选项标注启用/禁用状态，只排除当前池。
- 现有 `QuotaPool.Enabled` 有 GORM 默认值，测试创建 `Enabled: false` 的记录后须显式更新数据库字段为 `false`，否则实际保存的池可能仍为启用状态。

## rc.46 定制迁移（2026-10-23）

- 目标分支为 `v1.0.0-rc.46-icode`，基于 `v1.0.0-rc.46`（`7d8b91268`）；定制来源为 `v1.0.0-rc.25-icode`（`1deb4853b`）。原分支保持不变。
- [rc.25 功能映射](migration/rc25-icode-feature-map.md) 记录更早的 0.13.2 → rc.25 迁移历史；本次还包含 rc.25 之后的额度冻结、存量池普通化、Root 迁移、预算与统计、渠道并发、Magpie 监控和密钥导入等定制。
- 认证沿用 rc.46 的加密登录、会话、MFA、敏感操作 proof、PAT scope 和 Casbin。LDAP 复用密码登录的加密凭据与 `setupLogin`；额度冻结、池超级管理员任免和手动充值接入现有操作绑定校验，不能恢复旧版直接修改角色或状态的实现。
- 新增 `quota_pool:read/write` PAT scope，只允许访问角色与池归属原本允许的操作，不授予系统用户管理权限。
- 钉钉注册、可信邮箱归并和主动绑定统一通过外部身份 claim 事务。启动迁移会回填历史钉钉绑定；发现多个用户占用同一身份时明确失败，需要先人工修复重复绑定。邮箱归并遇到历史重复邮箱也明确失败。
- 保留 rc.46 的类型化 `AuditOther` / `LogOther`。额度池操作同时写入新的审计表和原额度池历史使用的日志表；写入专用审计后须标记请求已记录，避免鉴权中间件再写通用操作。独立日志库场景的测试与查询必须读取 `LOG_DB`。
- 消费和错误日志使用真实请求 UA，并保留元数据可见性边界；调用方传入的 UA 不能覆盖实际请求。上游响应模型观察优先复用 rc.46 已有实现，避免重复埋点。
- 保留 rc.46 的任务插件架构，移除旧 Ali 图片轮询器。任务查询尚未选路时不调用渠道上下文初始化；后续处理器绑定原任务渠道。渠道并发租约沿请求 context 传递，保持满载立即 429 的原定制行为。
- 按用户要求排除 Jev/TypeSafe 原生适配，保留 rc.46 任务插件机制；未安装或验证外部 Jev 插件。迁移测试仅用旧持久化编号 64 验证存量渠道不被重编号，旧原生渠道不能因此直接获得插件路由能力。
- 已补回隐藏首页文档按钮的展示定制，登录前后均隐藏，覆盖默认文档地址、自定义外链和站内路径；保留文档地址配置与项目归属。新迁入弹窗复用 `Dialog` / `ConfirmDialog`；迁池业务失败时保留选中目标并允许重试。
- 前端敏感词设置合入 rc.46 的请求策略页；侧边栏说明测试迁至安全功能测试目录。这些属于入口或测试位置迁移，不应重复恢复旧文件。

### 验证方法与边界

- 迁移回归 `TestICodeRC46DatabaseMigration` 使用 rc.25 定制分支的持久化字段快照，验证新库、旧库升级、两次迁移、身份唯一性及独立日志库数据保留。快照不得随当前模型一起修改。
- 该回归及 `TestQuotaPoolLegacyMigrationAcrossExternalDatabases` 会删除测试库中的表；只允许对专用可销毁测试库运行，必须显式设置 `TEST_QUOTA_POOL_DESTRUCTIVE=1`。
- 外部数据库使用 `TEST_MYSQL_DSN`、`TEST_POSTGRES_DSN`，独立日志库使用 `TEST_ICODE_MYSQL_LOG_DSN`、`TEST_ICODE_POSTGRES_LOG_DSN`。管理与安全回归使用 `TEST_MANAGE_USER_DIALECT`、`TEST_MANAGE_USER_SEPARATE_LOG_DB=1`、`TEST_SECURITY_DIALECT`；审计启动矩阵使用 `AUDIT_MYSQL_DSN`、`AUDIT_POSTGRES_DSN`。
- 认证审查参考 OWASP Authentication、Session Management、OAuth2 和 Cross-Site Request Forgery Prevention Cheat Sheets，回归覆盖加密凭据、禁用账号、MFA、会话失效、proof 绑定及重放、身份占用冲突。该检查不代表 ASVS 全项合规认证。

本轮验证以 2026-10-23 工作区为准；复用了历史 rc.46 适配提交 `b3cd7e987` 中仍适用的实现，并补入来源分支 `1deb4853b` 的禁用目标池迁移修复。历史适配的测试结果不作为本轮证据。

| 检查 | 本轮结果 |
| --- | --- |
| `GOMAXPROCS=2 go test -p=1 -parallel=2 ./...` | 49 个有测试包通过，34 个包无测试文件 |
| `GOMAXPROCS=2 GOWORK=off go build -p=1 ./...`（分别在 `relaykit/`、`tokenkit/`） | 两个模块独立构建通过 |
| 主程序构建（Go 并发 2，包并发 1） | 携带本轮前端产物构建通过 |
| `gofmt -l`（219 个迁移 Go 文件） | 无未格式化文件 |
| 禁用目标池迁移回归 | 后端先复现 `quota pool disabled`，前端先复现目标缺失；修复后 27 项后端相关测试、7 项原前端用例通过；另补迁移失败保留选择并允许重试的用例，已纳入全量前端测试 |
| `bun run test --maxWorkers=2` | 262 个测试文件、2716 项测试通过 |
| `GOMAXPROCS=2 bun run typecheck` | 通过 |
| `GOMAXPROCS=2 RAYON_NUM_THREADS=2 UV_THREADPOOL_SIZE=2 bun run build` | 通过 |
| 迁移范围 `oxlint --threads=2`（196 个代码文件） | 无 error；保留页脚既有 `dangerouslySetInnerHTML` warning |
| 七语种翻译增量检查 | 各新增 479 个键，上游原有键值未覆盖 |
| 前端格式、差异空白与冲突标记检查 | 通过，格式化保留版权头 |
| `bun run copyright:check` | 本次迁移文件无待修复项；全仓仍有 24 个未改动的上游文件未通过，未扩展修改 |
| 数据库新建/升级/重复迁移/身份唯一/独立日志库 | SQLite 3.50.4、PostgreSQL 17.11 通过；MySQL 未运行 |
| PostgreSQL 管理与认证矩阵 | 用户管理、额度调整、LDAP 登录、安全 enrollment、OAuth 邮箱归并与审计回归通过；使用专用本机数据库及 fixture |

数据库测试实例在本轮测试结束后已停止并清理，保留测试日志。MySQL 服务端与 Docker 服务当前不可用，因此 MySQL 子用例跳过；不能用历史 MySQL 通过记录替代本轮实测。当前未跑整仓前端 lint，校验范围为本次迁移代码。

重任务串行执行，Go 包并发为 1、进程并发为 2，Vitest/oxlint/oxfmt worker 或线程上限为 2。

生产数据库副本、真实 LDAP/Magpie/钉钉、ClickHouse 及 Jev 插件尚未验收。字段快照不能替代生产副本升级演练。
