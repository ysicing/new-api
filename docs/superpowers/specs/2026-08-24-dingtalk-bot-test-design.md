# 钉钉 Bot 测试与自动绑定设计

## 目标

简化企业内部应用机器人配置，并在系统设置中提供可指定平台用户的个人消息测试。未绑定钉钉的平台用户按企业约定使用邮箱前缀作为 staff userId，首次查询通讯录并校验邮箱后，将 unionId 保存到现有 `users.dingtalk_id`。

## 配置简化

- 删除 `DingTalkSettings.RobotCode`。
- 删除前端 Robot Code 输入框、设置类型和默认值。
- `IsRobotConfigured` 仅要求 Client ID 与 Client Secret。
- 机器人单聊请求的 `robotCode` 固定使用 Client ID/AppKey。
- 已持久化的 `dingtalk.robot_code` Option 保留但忽略，不删除数据、不增加迁移。

本项目只支持企业内部应用机器人；不覆盖自定义群机器人、群模板机器人或独立 Robot Code 场景。

## 用户解析与自动绑定

### staff userId 规则

企业约定 staff userId 等于平台邮箱 `@` 前缀，例如：

- 平台邮箱：`zhangsan@example.com`
- staff userId：`zhangsan`

邮箱缺失、格式无效或前缀为空时拒绝发送。

### 首次绑定

当平台用户 `dingtalk_id` 为空时：

1. 使用 Client ID/Client Secret 获取钉钉企业应用 AccessToken。
2. 按 staff userId 查询钉钉用户完整详情。
3. 要求返回的 `userId` 与邮箱前缀一致。
4. 要求返回邮箱规范化后与平台邮箱完全一致。
5. 要求返回 unionId 非空。
6. 事务内锁定平台用户，重新检查其绑定状态。
7. 检查该 unionId 未被其他有效或软删除平台账号占用。
8. 将 unionId 写入现有 `users.dingtalk_id`。
9. 发送机器人单聊消息给 staff userId。

已绑定相同 unionId视为幂等成功；已绑定其他 unionId 或 unionId 被其他账号占用时拒绝，不自动覆盖。

### 后续发送

平台用户已有 `dingtalk_id` 时，不再查询通讯录，直接根据当前邮箱前缀生成 staff userId 并发送。unionId 用于钉钉 OAuth 登录账号匹配，不作为机器人 API 接收人字段。

正式个人通知和测试消息共用同一接收人解析服务，彻底替换当前未经验证的“直接使用邮箱前缀”逻辑。

## 钉钉 API

机器人消息使用新版 API：

- 获取应用 Token：`POST /v1.0/oauth2/accessToken`
- 发送个人消息：`POST /v1.0/robot/oToMessages/batchSend`

首次绑定的用户详情使用企业应用通讯录 API：

- 获取旧版企业 Token：`GET /gettoken?appkey=...&appsecret=...`
- 查询用户详情：`POST /topapi/v2/user/get?access_token=...`

用户详情响应必须校验 HTTP 状态、`errcode`、userId、email 和 unionId。AccessToken 缓存按凭证隔离并提前一分钟过期；Token 失效时允许强制刷新后重试一次。

日志不得记录 Client Secret、AccessToken、带 token 的完整 URL 或完整上游响应。

## 测试用户搜索 API

新增 Root-only 接口：

### `GET /api/dingtalk/test-users`

查询参数：

- `keyword`：平台用户 ID、用户名、显示名、邮箱或部门。
- `p`、`page_size`：沿用项目分页规则，最大 50。

只返回：

- 状态启用；
- 邮箱规范化有效；
- 角色不是 Root 的平台用户。

响应字段：

- `id`
- `username`
- `display_name`
- `email`
- `department`
- `dingtalk_bound`：是否已有 unionId

## 测试消息 API

新增 Root-only 接口：

### `POST /api/dingtalk/test-message`

请求：

```json
{"user_id": 123}
```

流程：

1. 校验已保存的 Client ID 与 Client Secret。
2. 查询选中的启用平台用户。
3. 解析 staff userId；必要时查询通讯录并保存 unionId。
4. 发送 `sampleMarkdown` 个人机器人消息。
5. 写入 `dingtalk_notifications` 投递记录，事件类型为 `test`，每次使用唯一 dedupe key。
6. 返回 `bound_now` 表明本次是否自动绑定。

测试消息包含系统名称、平台用户及 ID、发送时间，以及“用于验证钉钉机器人个人通知配置”的说明。

稳定错误类别：

- 配置未完成
- 平台用户不存在或不可用
- 邮箱无效
- 通讯录 Token/权限/查询失败
- staff userId 或邮箱不一致
- unionId 缺失或绑定冲突
- 机器人 Token/权限/限流/发送失败

客户端不返回数据库错误、Client Secret、AccessToken 或上游完整响应。

## 系统设置 UI

钉钉认证设置：

- 保留启用、Corp ID、Client ID / AppKey、Client Secret / AppSecret、OAuth 回调地址。
- 删除 Robot Code。
- 增加可搜索用户选择器，只显示测试用户 API 返回的候选项。
- 候选项显示名称、邮箱、部门和“已绑定/未绑定”状态。
- 增加“保存并发送测试消息”按钮。

按钮流程：

1. 保存当前设置；Client Secret 留空时保留旧值。
2. 确认已选择测试用户。
3. 调用测试消息 API。
4. 成功提示“测试消息发送成功”；`bound_now=true` 时追加“已通过企业邮箱绑定钉钉账号”。
5. 失败时保留当前选择和表单内容。

保存按钮与测试按钮分别保留。请求期间禁用重复提交，按钮显示加载状态。

## 测试与验收

### 后端

- Robot payload 使用 Client ID，不再读取 Robot Code。
- 仅 Client ID/Client Secret 即可判定机器人已配置。
- 已绑定用户不查询通讯录。
- 未绑定用户按邮箱前缀查询，邮箱和 userId 完全一致时保存 unionId。
- 邮箱不一致、unionId 冲突、用户已有其他绑定时不写数据库、不发送。
- 并发重复绑定同一用户幂等。
- 正式通知和测试消息共用解析服务。
- Root-only 路由拒绝 Admin、池超级管理员和普通用户。
- 测试消息每次可发送并写入独立投递记录。

### 前端

- Robot Code 不再出现在设置值、表单和保存请求中。
- 用户选择器支持搜索、加载、空状态和错误状态。
- 未选用户时不能发送测试消息。
- “保存并测试”按先保存后测试顺序执行。
- 成功、自动绑定和错误提示正确。
- 移动端表单和用户选择器无横向溢出。
- `en`、`zh`、`zh-TW`、`fr`、`ru`、`ja`、`vi` 无缺失文案。

### 最终验证

- 钉钉 service/model/controller/router 相关测试与后端全量测试。
- 前端钉钉设置测试、全量测试、typecheck、格式、涉及文件 lint、i18n 同步和生产构建。
- `git diff --check`，确认无临时脚本和构建产物进入提交。

## 非目标

- 不支持自定义群机器人或独立 Robot Code。
- 不新增 staff userId 数据库字段。
- 不按姓名、部门或模糊邮箱自动绑定钉钉账号。
- 不覆盖用户已有的不同 unionId。
- 不删除数据库已有的 `dingtalk.robot_code` Option。
