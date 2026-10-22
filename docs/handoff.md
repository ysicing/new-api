# 开发交接注意事项

## 额度池与 Root 账号

- `IsQuotaPoolMemberRole` 同时用于成员添加和池管理员任命，不能仅为放开 Root 迁移就把 Root 加入该角色集合。Root 迁移通过 `MoveUserBetweenQuotaPools` 单独开放，并在事务内核实操作人的 Root 身份。
- 迁移只改变额度和池归属，保留系统角色；禁用池仍禁止 Root 手动充值和自动充值，迁出时照常回收余额并记录流水。
- 现有 `QuotaPool.Enabled` 有 GORM 默认值，测试创建 `Enabled: false` 的记录后须显式更新数据库字段为 `false`，否则实际保存的池可能仍为启用状态。
