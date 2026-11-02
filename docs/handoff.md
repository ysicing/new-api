# 开发交接注意事项

## 额度池与 Root 账号

- `IsQuotaPoolMemberRole` 同时用于成员添加和池管理员任命，不能仅为放开 Root 迁移就把 Root 加入该角色集合。Root 迁移通过 `MoveUserBetweenQuotaPools` 单独开放，并在事务内核实操作人的 Root 身份。
- 迁移只改变额度和池归属，保留系统角色；禁用池仍禁止 Root 手动充值和自动充值，迁出时照常回收余额并记录流水。
- 现有 `QuotaPool.Enabled` 有 GORM 默认值，测试创建 `Enabled: false` 的记录后须显式更新数据库字段为 `false`，否则实际保存的池可能仍为启用状态。
- 用户管理的额度池迁移允许迁入禁用池；目标列表只排除当前池，并显示启用状态。不能把充值的启用校验用于成员归属迁移：迁移照常回收原池余额，禁用目标池仍禁止手动和自动发放额度。

## DeepSeek Search/Fetch 与推理修饰符

- `/v1/tools/search`、`/v1/tools/fetch` 分别按 `deepseek-search`、`deepseek-fetch` 选择渠道和定价，请在渠道模型列表中添加对应名称。请求体的 `model` 不参与选择或计费；原始字段照常转发。
- 独立 Search/Fetch 默认每次成功返回结果收取 $0.00075；空结果、空页面和失败响应释放预扣。Claude/Responses 内部搜索、抓取使用 `deepseek_search`、`deepseek_fetch` 工具价格（默认 $0.75/千次），不随映射后的模型别名切换到其他供应商价格。
- Flash/Pro 默认使用北京时间工作日 09:00–12:00、14:00–18:00 的峰谷表达式；未内置中国法定节假日日历，节假日工作日按峰时处理。管理员已有模型价格、倍率、计费模式优先，若需节假日精确定价，应自行配置表达式。
- DeepSeek 支持 `-none/-low/-max` 和 `@thinking:on|off|adaptive|预算`、`@effort:等级`，显式修饰符优先于后缀及请求字段。修饰符在 Chat、Claude、Responses 中渲染为 DeepSeek 原生字段；Claude 的其他 `output_config` 成员保留。`ThinkingModelBlacklist` 中的完整模型名保持原样。
