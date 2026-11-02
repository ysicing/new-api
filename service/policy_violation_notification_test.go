package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyViolationNotificationIsRateLimitedAndGivesReviewGuidance(t *testing.T) {
	db := setupDingTalkNotificationServiceDB(t)
	user := model.User{
		Username: "policy-notify-user", Email: "policy-notify-user@example.com",
		Password: "password", AffCode: "policy-notify-aff", Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&user).Error)
	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	*settings = system_setting.DingTalkSettings{}
	t.Cleanup(func() {
		*settings = previousSettings
	})
	detectedAt := time.Date(2026, time.August, 25, 10, 15, 0, 0, time.Local)

	require.NoError(t, notifyPolicyViolationDetectedAt(user.Id, detectedAt))
	require.NoError(t, notifyPolicyViolationDetectedAt(user.Id, detectedAt.Add(20*time.Minute)))

	var records []model.DingTalkNotification
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, model.DingTalkNotificationEventPolicyViolation, records[0].EventType)
	assert.Equal(t, policyViolationNotificationTitle, records[0].Title)
	assert.Equal(t, "您的请求被判定为违反使用条款或使用策略。请勿重试当前请求，先复核请求内容，确认具体违反了哪项条款或策略，并完成调整后再发起新请求。", records[0].Content)
	assert.NotContains(t, records[0].Content, "iCode")
	assert.NotContains(t, records[0].Content, "查看使用日志")
}
