package service

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	newUserQuotaExhaustedTitle   = "体验额度已用完"
	newUserQuotaExhaustedContent = "当前体验额度已经用完，请联系部门额度池管理员及时添加到对应额度池。"
)

func checkAndSendNewUserQuotaExhaustedNotify(relayInfo *relaycommon.RelayInfo, consumeQuota int) {
	gopool.Go(func() {
		if err := notifyNewUserQuotaExhausted(relayInfo, consumeQuota); err != nil {
			common.SysError(fmt.Sprintf("failed to send new-user quota notification to user %d: %s", relayInfo.UserId, err.Error()))
		}
	})
}

// notifyNewUserQuotaExhausted 仅在新用户额度池成员的余额首次跨越到零时创建通知。
func notifyNewUserQuotaExhausted(relayInfo *relaycommon.RelayInfo, consumeQuota int) error {
	if relayInfo == nil || relayInfo.BillingSource == BillingSourceSubscription || consumeQuota <= 0 {
		return nil
	}
	quotaBefore := relayInfo.UserQuota
	quotaAfter := quotaBefore - consumeQuota
	if quotaBefore <= 0 || quotaAfter > 0 {
		return nil
	}
	user, err := model.GetUserById(relayInfo.UserId, false)
	if err != nil {
		return err
	}
	// 非批量扣费模式下以结算后的数据库余额再次确认，避免并发充值后误报额度耗尽。
	if !common.BatchUpdateEnabled && user.Quota > 0 {
		return nil
	}
	pool, err := model.GetQuotaPoolById(user.QuotaPoolId)
	if err != nil {
		return err
	}
	if !pool.IsNewUserPool() {
		return nil
	}
	_, err = DispatchDingTalkNotification(context.Background(), DingTalkNotificationRequest{
		EventType: model.DingTalkNotificationEventNewUserQuotaExhausted,
		DedupeKey: fmt.Sprintf("%s:%d", model.DingTalkNotificationEventNewUserQuotaExhausted, user.Id),
		UserId:    user.Id, UserEmail: user.Email,
		Title: newUserQuotaExhaustedTitle, Content: newUserQuotaExhaustedContent,
		Metadata: map[string]any{
			"pool_id": pool.Id, "pool_type": pool.PoolType,
			"quota_before": quotaBefore, "quota_after": quotaAfter,
		},
	})
	return err
}
