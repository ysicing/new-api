package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	policyViolationNotificationHourlyLimit = 1
	policyViolationNotificationTitle       = "请求违反使用政策"
)

func policyViolationNotificationContent() string {
	return "您的请求被判定为违反使用条款或使用策略。请勿重试当前请求，先复核请求内容，确认具体违反了哪项条款或策略，并完成调整后再发起新请求。"
}

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
		Title: policyViolationNotificationTitle, Content: policyViolationNotificationContent(),
		Metadata: map[string]any{"window_start": hourStart.Unix(), "window_end": hourStart.Add(time.Hour).Unix()},
	}, detectedAt, policyViolationNotificationHourlyLimit)
	return err
}
