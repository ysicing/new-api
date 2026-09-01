package service

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
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
	previousContactMessage := setting.SensitiveWordContactMessage
	previousServerAddress := system_setting.ServerAddress
	*settings = system_setting.DingTalkSettings{}
	setting.SensitiveWordContactMessage = ""
	system_setting.ServerAddress = ""
	t.Cleanup(func() {
		*settings = previousSettings
		setting.SensitiveWordContactMessage = previousContactMessage
		system_setting.ServerAddress = previousServerAddress
	})
	return &user, time.Date(2026, time.August, 25, 10, 15, 0, 0, time.Local)
}

func TestSensitiveWordNotificationIncludesFilteredUsageLogLink(t *testing.T) {
	user, detectedAt := setupSensitiveWordNotificationTest(t)
	system_setting.ServerAddress = "https://icode.51talk.biz/"

	require.NoError(t, notifySensitiveWordsDetectedAt(user.Id, []string{"link-word"}, detectedAt))

	var record model.DingTalkNotification
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&record).Error)
	const linkPrefix = "[查看使用日志]("
	linkStart := strings.Index(record.Content, linkPrefix)
	require.GreaterOrEqual(t, linkStart, 0)
	linkValue := record.Content[linkStart+len(linkPrefix):]
	linkEnd := strings.Index(linkValue, ")")
	require.Greater(t, linkEnd, 0)
	parsed, err := url.Parse(linkValue[:linkEnd])
	require.NoError(t, err)
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "icode.51talk.biz", parsed.Host)
	assert.Equal(t, "/usage-logs/common", parsed.Path)
	dayStart := time.Date(detectedAt.Year(), detectedAt.Month(), detectedAt.Day(), 0, 0, 0, 0, detectedAt.Location())
	assert.Equal(t, strconv.FormatInt(dayStart.UnixMilli(), 10), parsed.Query().Get("startTime"))
	assert.Equal(t, strconv.FormatInt(detectedAt.UnixMilli(), 10), parsed.Query().Get("endTime"))
	assert.Equal(t, `["5"]`, parsed.Query().Get("type"))
	assert.Equal(t, "1", parsed.Query().Get("page"))
}

func TestSensitiveWordNotificationDeduplicatesSameWordWithinHourWithoutPersistingPlaintext(t *testing.T) {
	user, detectedAt := setupSensitiveWordNotificationTest(t)

	require.NoError(t, notifySensitiveWordsDetectedAt(user.Id, []string{"classified-term", "classified-term"}, detectedAt))

	var records []model.DingTalkNotification
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, model.DingTalkNotificationEventSensitiveWordDetected, records[0].EventType)
	assert.Equal(t, "请求触发敏感词审查", records[0].Title)
	assert.Equal(t, "您的请求触发敏感词审查，请登录 iCode 在使用日志里查询错误类型日志。\n\n请先自查敏感词后再尝试提交请求，避免影响使用体验。", records[0].Content)
	persisted := strings.Join([]string{records[0].DedupeKey, records[0].Title, records[0].Content, records[0].Metadata, records[0].Error}, " ")
	assert.NotContains(t, persisted, "classified-term")
}

func TestSensitiveWordNotificationAppendsConfiguredContactMessage(t *testing.T) {
	user, detectedAt := setupSensitiveWordNotificationTest(t)
	require.NoError(t, setting.SetSensitiveWordContactMessage("  如有误判，请通过钉钉联系张三处理。  "))

	require.NoError(t, notifySensitiveWordsDetectedAt(user.Id, []string{"contact-word"}, detectedAt))

	var record model.DingTalkNotification
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&record).Error)
	assert.Equal(t, "您的请求触发敏感词审查，请登录 iCode 在使用日志里查询错误类型日志。\n\n请先自查敏感词后再尝试提交请求，避免影响使用体验。\n\n如有误判，请通过钉钉联系张三处理。", record.Content)
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
