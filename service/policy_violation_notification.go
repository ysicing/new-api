package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const policyViolationNotificationHourlyLimit = 1

// NotifyPolicyViolationDetected 异步发送使用政策违规通知，不传递请求内容。
func NotifyPolicyViolationDetected(userId int) {
	if userId <= 0 {
		return
	}
	detectedAt := time.Now()
	gopool.Go(func() {
		if err := notifyPolicyViolationDetectedAt(userId, detectedAt); err != nil {
			common.SysError(fmt.Sprintf("failed to send policy-violation DingTalk notification to user %d: %s", userId, err.Error()))
		}
	})
}

func notifyPolicyViolationDetectedAt(userId int, detectedAt time.Time) error {
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return err
	}
	hourStart := time.Date(detectedAt.Year(), detectedAt.Month(), detectedAt.Day(), detectedAt.Hour(), 0, 0, 0, detectedAt.Location())
	_, _, err = dispatchHourlyRateLimitedDingTalkNotification(context.Background(), DingTalkNotificationRequest{
		EventType: model.DingTalkNotificationEventPolicyViolation,
		DedupeKey: fmt.Sprintf("%s:%d:%d", model.DingTalkNotificationEventPolicyViolation, user.Id, hourStart.Unix()),
		UserId:    user.Id, UserEmail: user.Email,
		Title:    i18n.Translate(user.GetSetting().Language, i18n.MsgNotifyPolicyViolationTitle),
		Content:  i18n.Translate(user.GetSetting().Language, i18n.MsgNotifyPolicyViolationContent),
		Metadata: map[string]any{"window_start": hourStart.Unix(), "window_end": hourStart.Add(time.Hour).Unix()},
	}, detectedAt, policyViolationNotificationHourlyLimit)
	return err
}
