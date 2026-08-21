# rc.25 迁移 0.13.2-icode 定制功能实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在官方 `v1.0.0-rc.25` 基线上完整迁移 `v0.13.2-icode` 定制功能，并兼容旧生产数据库原地升级。

**Architecture:** 按 Router、Controller、Service、Model 分层重写旧定制能力；额度池仅负责向用户钱包发放额度，不介入请求计费。数据库迁移保持增量和幂等，前端按 rc.25 的 TypeScript、TanStack Router 和 feature 结构原生实现。

**Tech Stack:** Go 1.25、Gin、GORM、SQLite/MySQL/PostgreSQL、ClickHouse 日志库、React 19、TypeScript、TanStack Router、shadcn/ui、Bun、Vitest。

---

### Task 1: 功能映射与升级基准

- [ ] 记录 64 个旧分支提交的迁移状态和验收入口。
- [ ] 建立旧数据库 fixture 和迁移前后数据摘要断言。
- [ ] 运行 SQLite fixture 测试并提交文档及测试基准。

### Task 2: 模型、配置与幂等迁移

- [ ] 先编写旧 schema 重复迁移和新安装默认值的失败测试。
- [ ] 增加额度池模型、用户字段、配置和迁移初始化。
- [ ] 验证 SQLite、MySQL、PostgreSQL 迁移语义并提交。

### Task 3: 权限与额度池领域服务

- [ ] 先编写资金原子性、成员迁移和角色权限失败测试。
- [ ] 实现拆分后的额度池模型操作和 Service 规则。
- [ ] 验证并发、回滚、capability 和越权场景并提交。

### Task 4: HTTP API 与错误契约

- [ ] 先编写旧路由兼容、零值参数、warning 和错误码测试。
- [ ] 接入额度池管理、自助、用户管理和统计接口。
- [ ] 补齐后端 i18n 和响应契约并提交。

### Task 5: 自动充值与月度补齐

- [ ] 先编写次数限制、并发执行、补齐和租约接管测试。
- [ ] 实现 `quota_pool_maintenance` SystemTask。
- [ ] 验证旧日志兼容和任务隔离并提交。

### Task 6: LDAP 与 OAuth 归并

- [ ] 先编写 LDAP、2FA、session、签名和 OAuth 冲突测试。
- [ ] 增加 LDAP 依赖、服务、配置和接口。
- [ ] 实现可信已验证邮箱归并并提交。

### Task 7: 统计、IP 日志与审计

- [ ] 先编写独立日志库、排序和零占比测试。
- [ ] 实现两阶段统计聚合和 ClickHouse 分支。
- [ ] 启用 IP 记录并完善资金、操作和任务审计后提交。

### Task 8: 新版前端与国际化

- [ ] 使用 ui-ux-pro-max、shadcn-ui 和 React best practices 规划组件边界。
- [ ] 先编写路由、capability、登录和表单组件测试。
- [ ] 实现额度池、用户管理、设置、Dashboard、LDAP 登录和版本展示。
- [ ] 使用 i18n-translate 补齐七种语言并提交。

### Task 9: 整合验证与交付

- [ ] 逐项关闭功能映射清单。
- [ ] 执行前端、后端、race、三数据库、ClickHouse 和升级验证。
- [ ] 执行 code review，修复确认问题并重新验证。
- [ ] 检查完整工作区并输出交付总结；未经授权不 push、不创建 PR。
