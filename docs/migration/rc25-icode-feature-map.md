# rc.25 / 0.13.2-icode 功能迁移映射

## 判定口径

- `migrate`：已在 rc.25 架构中实现，等待全部外部验收门禁关闭后统一标记为 `done`。
- `upstream`：补丁已经由 rc.25 等价吸收，不重复迁移。
- 每个 `migrate` 项只有在对应测试和功能验收通过后才能改为 `done`。

## 提交映射

| Commit | Domain | Status | Target / acceptance |
| --- | --- | --- | --- |
| `fa111fe0a` | 自动充值、充值榜 | migrate | SystemTask、统计 API、管理 UI |
| `012437c4a` | IP 日志 | migrate | 消费和错误日志始终记录 IP |
| `671f0579d` | 子管理员权限 | migrate | Admin 负向权限测试 |
| `ddd4aaf8e` | 用户额度权限 | migrate | 非 Root 禁止任意调额 |
| `4a7bfec9e` | Dashboard | migrate | 运营统计入口始终可用 |
| `a24e93895` | Top 用户 | migrate | 时间、模型和 limit 聚合 |
| `25b633413` | Top 用户 UI | migrate | 用量与余额展示 |
| `23f70e4f1` | 统计过滤 | migrate | 隐藏零占比模型 |
| `5d36c27c3` | 充值榜 | migrate | 模型占比统计 |
| `a4a83f331` | 用户 UI 权限 | migrate | capability 隐藏操作 |
| `9225f0493` | 充值榜排序 | migrate | 次数并列稳定排序 |
| `6507e69eb` | 充值审计 | migrate | 记录管理员信息 |
| `6281b9992` | 额度池设计 | done | 已合并到迁移设计文档 |
| `46aaa4b5f` | 额度池核心 | migrate | 模型、资金、成员、权限 |
| `057613d00` | 额度池体验 | migrate | 管理及自助 API/UI |
| `0424d0d37` | 池统计和日志 | migrate | 统计、流水、日志查询 |
| `ce1ac5eba` | 候选成员 | migrate | 候选查询和测试 |
| `d73955aef` | 管理员角色 | migrate | v1/v2 和系统角色规则 |
| `e738444ee` | 默认池同步 | migrate | 幂等系统池初始化 |
| `ed821e8ed` | 用户角色管理 | migrate | 超级管理员设置和缓存失效 |
| `080544560` | 池统计 UI | migrate | 新版详情统计区 |
| `3ac406943` | 角色文案 | migrate | 七语种角色标签 |
| `9b7e0c4ea` | LDAP | migrate | 登录、同步、设置、用户字段 |
| `800b32de8` | 池自动充值配置 | migrate | 系统及池级策略 |
| `51bcf6a1d` | 登录 UI | migrate | LDAP 登录入口和响应 |
| `4c977ab02` | 用户资料 | migrate | 部门、绑定和错误处理 |
| `71f77d2a6` | 额度回收 | migrate | 回收选项和事务校验 |
| `05cbf7d1c` | 池易用性 | migrate | 权限驱动操作界面 |
| `85b2045e3` | 自助池配置 | migrate | `/api/quota_pool/self` |
| `b5138c3ad` | 池额度调整 | migrate | base quota 调整和限制 |
| `736821948` | 池权限审计 | migrate | capability 与操作日志 |
| `5429f5065` | 用户池名称 | migrate | 用户响应和列表列 |
| `a7aabf4c1` | LDAP 单选同步 | migrate | 签名候选项流程 |
| `f103fc047` | 池角色 UI | migrate | 角色标签和操作权限 |
| `d4c4691ea` | LDAP 查询 | migrate | 同步不重复查询 |
| `245b9432c` | LDAP 空邮箱 | migrate | LDAP ID 回退匹配 |
| `ee74e5e31` | 流水和统计 | migrate | 筛选、排序和展示 |
| `36670b566` | 成员识别 | migrate | 用户名、邮箱、部门候选信息 |
| `02a404bd8` | OAuth 邮箱归并 | migrate | 仅可信已验证邮箱自动归并 |
| `fcd687f72` | 用户池归属 | migrate | 成员所在池展示 |
| `af8a69f2d` | 池管理员联系方式 | migrate | 联系信息响应与 UI |
| `4056c7825` | 余额不足提示 | migrate | 钱包不足时联系池管理员 |
| `9d4bd4d49` | 新用户池 | migrate | 幂等初始化和注册归属 |
| `1b6abc147` | 池用量统计 | migrate | 独立 LOG_DB 两阶段聚合 |
| `5485f129c` | 池操作日志 | migrate | 操作日志 API/UI |
| `5118d59bd` | 默认池名称 | migrate | 系统池固定语义 |
| `4d44842aa` | 日志文案 | migrate | 结构化来源及统一文案 |
| `19cedfb72` | 用户池调整 | migrate | 用户管理迁移入口 |
| `40c7fbc06` | 构建版本 | migrate | 页脚环境变量版本 |
| `afd1933fa` | 版本来源 | migrate | 不回退到伪造版本 |
| `c39a87c7a` | 池筛选 | migrate | 用户列表 `quota_pool_id` 筛选 |
| `f91081a4c` | 分级迁移 | migrate | 默认、新用户、普通池迁移 |
| `f23702c3a` | 自动充值校验 | migrate | 阈值、金额和风险 warning |
| `499297924` | 用户降额 | migrate | 100%/50%/40%/30%/20%/10% |
| `485a1083f` | 系统池只读 | migrate | 默认池和新用户池保护 |
| `07e3aaca7` | i18n | migrate | 七语种完整性检查 |
| `d35e11775` | 首充失败 | migrate | 迁移成功并返回 warning |
| `409416aec` | 月度补齐 | migrate | fixed/top-up 两种模式 |
| `d887dd73d` | 周充值统计 | migrate | 个人周次数及剩余次数 |
| `336f5bf10` | 模型倍率 | upstream | rc.25 已包含等价补丁 |
| `e7b7822e8` | 跨角色充值 | migrate | 充值不受目标角色等级限制 |
| `1ddca5b9b` | 工具调用索引 | upstream | rc.25 已包含等价补丁 |
| `09fb2d21b` | 空思考内容 | upstream | rc.25 已包含等价补丁 |
| `b48f6e247` | 渠道测试用户 ID | upstream | rc.25 已包含等价补丁 |

## 验收汇总

- 总计：64 个提交。
- 已由上游吸收：4 个。
- 已完成设计归并：1 个。
- 已实现、待外部验收：59 个。

## 实施状态

- 59 个 `migrate` 项均已落入本分支的模型迁移、额度池、自动充值、认证、统计或新版前端提交。
- SQLite 全量测试和旧库重复迁移已通过。
- PostgreSQL 15 独立实例旧库重复迁移和数据不变量检查已通过。
- MySQL 8 因本机 Docker 凭据助手无法拉取镜像，尚未完成真实实例验证。
- ClickHouse 独立日志库和生产数据库副本升级属于上线前外部验收门禁，当前环境未提供可用实例或副本。
