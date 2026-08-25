package service

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSensitiveWordNotificationTest(t *testing.T) (*model.User, time.Time) {
	t.Helper()
	db := setupDingTalkNotificationServiceDB(t)
	user := model.User{
		Username: "sensitive-notify-user", Email: "sensitive-notify-user@example.com",
		Password: "password", AffCode: "sensitive-notify-aff", Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&user).Error)
	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	*settings = system_setting.DingTalkSettings{}
	t.Cleanup(func() { *settings = previousSettings })
	return &user, time.Date(2026, time.August, 25, 10, 15, 0, 0, time.Local)
}

func TestSensitiveWordNotificationDeduplicatesSameWordWithinHourWithoutPersistingPlaintext(t *testing.T) {
	user, detectedAt := setupSensitiveWordNotificationTest(t)

	require.NoError(t, notifySensitiveWordsDetectedAt(user.Id, []string{"classified-term", "classified-term"}, detectedAt))

	var records []model.DingTalkNotification
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, model.DingTalkNotificationEventSensitiveWordDetected, records[0].EventType)
	assert.Equal(t, "请求触发敏感词审查", records[0].Title)
	assert.Equal(t, "您的请求触发敏感词审查，请登录 iCode 在使用日志里查询错误类型日志。\n\n请先自查敏感词后再尝试提交请求，避免影响使用体验。如有误判,请联系管理员处理", records[0].Content)
	persisted := strings.Join([]string{records[0].DedupeKey, records[0].Title, records[0].Content, records[0].Metadata, records[0].Error}, " ")
	assert.NotContains(t, persisted, "classified-term")
}

func TestSensitiveWordNotificationCapsDistinctWordsAtThreePerUserHour(t *testing.T) {
	user, detectedAt := setupSensitiveWordNotificationTest(t)

	require.NoError(t, notifySensitiveWordsDetectedAt(user.Id, []string{"word-a", "word-b", "word-c", "word-d", "word-e"}, detectedAt))

	var records []model.DingTalkNotification
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).Order("id").Find(&records).Error)
	require.Len(t, records, 4)
	for _, record := range records[:3] {
		assert.Equal(t, "DingTalk robot is not configured", record.Error)
	}
	assert.Equal(t, "DingTalk notification hourly limit exceeded", records[3].Error)
}

func TestSensitiveWordNotificationResetsNextHourAndIsolatesUsers(t *testing.T) {
	firstUser, detectedAt := setupSensitiveWordNotificationTest(t)
	secondUser := model.User{
		Username: "second-sensitive-user", Email: "second-sensitive-user@example.com",
		Password: "password", AffCode: "second-sensitive-aff", Status: common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&secondUser).Error)

	require.NoError(t, notifySensitiveWordsDetectedAt(firstUser.Id, []string{"same-word"}, detectedAt))
	require.NoError(t, notifySensitiveWordsDetectedAt(firstUser.Id, []string{"same-word"}, detectedAt.Add(time.Hour)))
	require.NoError(t, notifySensitiveWordsDetectedAt(secondUser.Id, []string{"same-word"}, detectedAt))

	var firstCount, secondCount int64
	require.NoError(t, model.DB.Model(&model.DingTalkNotification{}).Where("user_id = ?", firstUser.Id).Count(&firstCount).Error)
	require.NoError(t, model.DB.Model(&model.DingTalkNotification{}).Where("user_id = ?", secondUser.Id).Count(&secondCount).Error)
	assert.EqualValues(t, 2, firstCount)
	assert.EqualValues(t, 1, secondCount)
}
