package service

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

// NotifyNewUserRegistered 在自助注册完成后异步发送欢迎消息，通知失败不影响注册结果。
func NotifyNewUserRegistered(userId int) {
	gopool.Go(func() {
		user, err := model.GetUserById(userId, false)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to load registered user %d for DingTalk notification: %s", userId, err.Error()))
			return
		}
		_, err = DispatchDingTalkNotification(context.Background(), DingTalkNotificationRequest{
			EventType: model.DingTalkNotificationEventUserRegistered,
			DedupeKey: fmt.Sprintf("%s:%d", model.DingTalkNotificationEventUserRegistered, user.Id),
			UserId:    user.Id, UserEmail: user.Email,
			Title:    i18n.Translate(user.GetSetting().Language, i18n.MsgNotifyUserRegisteredTitle),
			Content:  i18n.Translate(user.GetSetting().Language, i18n.MsgNotifyUserRegisteredContent),
			Metadata: map[string]any{"quota_pool_id": user.QuotaPoolId},
		})
		if err != nil {
			common.SysError(fmt.Sprintf("failed to send registration DingTalk notification to user %d: %s", user.Id, err.Error()))
		}
	})
}

// NotifyQuotaPoolJoined 在用户从新用户额度池加入普通额度池后异步发送通知。
func NotifyQuotaPoolJoined(userId, poolId int) {
	gopool.Go(func() {
		user, err := model.GetUserById(userId, false)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to load quota-pool member %d for DingTalk notification: %s", userId, err.Error()))
			return
		}
		pool, err := model.GetQuotaPoolById(poolId)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to load quota pool %d for DingTalk notification: %s", poolId, err.Error()))
			return
		}
		if pool.IsSystemPool() || pool.PoolType != model.QuotaPoolTypeNormal {
			return
		}
		_, err = DispatchDingTalkNotification(context.Background(), DingTalkNotificationRequest{
			EventType: model.DingTalkNotificationEventQuotaPoolJoined,
			DedupeKey: fmt.Sprintf("%s:%d:%d:%s", model.DingTalkNotificationEventQuotaPoolJoined, user.Id, pool.Id, common.GetUUID()),
			UserId:    user.Id, UserEmail: user.Email,
			Title:    i18n.Translate(user.GetSetting().Language, i18n.MsgNotifyQuotaPoolJoinedTitle),
			Content:  i18n.Translate(user.GetSetting().Language, i18n.MsgNotifyQuotaPoolJoinedContent, map[string]any{"PoolName": pool.Name}),
			Metadata: map[string]any{"pool_id": pool.Id, "pool_name": pool.Name},
		})
		if err != nil {
			common.SysError(fmt.Sprintf("failed to send quota-pool DingTalk notification to user %d: %s", user.Id, err.Error()))
		}
	})
}
