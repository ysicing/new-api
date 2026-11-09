package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAnnouncementGroupNotificationTest(t *testing.T) {
	t.Helper()
	db := setupDingTalkNotificationServiceDB(t)
	require.NoError(t, db.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))
	settings := system_setting.GetDingTalkSettings()
	previous := *settings
	*settings = system_setting.DingTalkSettings{
		ClientId: "app-key", ClientSecret: "app-secret",
		AnnouncementGroupOpenConversationId: "cid-announcement",
	}
	t.Cleanup(func() { *settings = previous })
}

func TestSyncAnnouncementGroupNotificationsCreatesUpdatesAndCancelsPending(t *testing.T) {
	setupAnnouncementGroupNotificationTest(t)
	now := time.Date(2026, time.September, 1, 10, 0, 0, 0, time.Local)
	previous := `[{"id":1,"content":"已有公告","publishDate":"2026-09-01T08:00:00+08:00","type":"default"}]`
	current := `[{"id":1,"content":"已有公告","publishDate":"2026-09-01T08:00:00+08:00","type":"default"},{"id":2,"content":"**Markdown** <b>HTML</b> <a href=\"https://example.com/docs\">文档</a> <img src=\"https://example.com/a.png\" alt=\"图片\">","extra":"补充说明","publishDate":"2026-09-01T12:00:00+08:00","type":"warning"}]`

	require.NoError(t, SyncAnnouncementGroupNotifications(previous, current, 9, "root", now))
	var notificationCount int64
	require.NoError(t, model.DB.Model(&model.DingTalkNotification{}).Count(&notificationCount).Error)
	assert.EqualValues(t, 1, notificationCount)
	var record model.DingTalkNotification
	require.NoError(t, model.DB.Where("dedupe_key = ?", "announcement_group:2").First(&record).Error)
	assert.Equal(t, model.DingTalkNotificationStatusPending, record.Status)
	assert.Equal(t, "cid-announcement", record.Recipient)
	assert.Equal(t, time.Date(2026, time.September, 1, 12, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)).Unix(), record.ScheduledAt)
	assert.Contains(t, record.Content, "**Markdown** HTML")
	assert.NotContains(t, record.Content, "<b>")
	assert.Contains(t, record.Content, "[文档](https://example.com/docs)")
	assert.Contains(t, record.Content, "![图片](https://example.com/a.png)")
	assert.Contains(t, record.Content, "补充说明")

	updated := `[{"id":1,"content":"已有公告","publishDate":"2026-09-01T08:00:00+08:00","type":"default"},{"id":2,"content":"修改内容","publishDate":"2026-09-01T13:00:00+08:00","type":"warning"}]`
	require.NoError(t, SyncAnnouncementGroupNotifications(current, updated, 9, "root", now))
	require.NoError(t, model.DB.Where("dedupe_key = ?", "announcement_group:2").First(&record).Error)
	assert.Equal(t, "修改内容\n\n发布时间：2026-09-01 13:00", record.Content)

	require.NoError(t, SyncAnnouncementGroupNotifications(updated, previous, 9, "root", now))
	require.NoError(t, model.DB.Where("dedupe_key = ?", "announcement_group:2").First(&record).Error)
	assert.Equal(t, model.DingTalkNotificationStatusSkipped, record.Status)
}

func TestSyncAnnouncementGroupNotificationsFormatsPublishTimeInBeijing(t *testing.T) {
	setupAnnouncementGroupNotificationTest(t)
	now := time.Date(2026, time.September, 1, 6, 0, 0, 0, time.UTC)
	current := `[{"id":1,"content":"UTC 时间公告","publishDate":"2026-09-01T05:00:00Z","type":"default"}]`

	require.NoError(t, SyncAnnouncementGroupNotifications("", current, 9, "root", now))
	var record model.DingTalkNotification
	require.NoError(t, model.DB.Where("dedupe_key = ?", "announcement_group:1").First(&record).Error)
	assert.Equal(t, "UTC 时间公告\n\n发布时间：2026-09-01 13:00", record.Content)
}

func TestDeliverDueAnnouncementGroupNotificationsFailsWithoutRetry(t *testing.T) {
	setupAnnouncementGroupNotificationTest(t)
	record := &model.DingTalkNotification{
		EventType: model.DingTalkNotificationEventAnnouncementGroup,
		DedupeKey: "announcement_group:3", Recipient: "cid-announcement",
		Title: "系统公告", Content: "公告正文", ScheduledAt: 100,
	}
	_, err := model.UpsertPendingAnnouncementGroupNotification(record)
	require.NoError(t, err)
	previousSend := sendAnnouncementGroupMessage
	calls := 0
	sendAnnouncementGroupMessage = func(context.Context, system_setting.DingTalkSettings, string, string, string) error {
		calls++
		return errors.New("send failed")
	}
	t.Cleanup(func() { sendAnnouncementGroupMessage = previousSend })

	result, err := deliverDueAnnouncementGroupNotifications(context.Background(), 100)

	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, 1, result.Attempted)
	assert.Equal(t, 1, result.Failed)
	require.NoError(t, model.DB.Where("dedupe_key = ?", record.DedupeKey).First(record).Error)
	assert.Equal(t, model.DingTalkNotificationStatusFailed, record.Status)
	assert.Equal(t, "send failed", record.Error)
	due, err := model.ListDueAnnouncementGroupNotifications(200, 10)
	require.NoError(t, err)
	assert.Empty(t, due)
}

func TestDeliverDueAnnouncementGroupNotificationsStopsClaimingAfterCancellation(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		canceledBefore bool
		sendError      error
		wantResult     announcementGroupDeliveryResult
		wantStatus     string
	}{
		{
			name: "already canceled", canceledBefore: true,
			wantStatus: model.DingTalkNotificationStatusPending,
		},
		{
			name:       "canceled during successful send",
			wantResult: announcementGroupDeliveryResult{Attempted: 1, Succeeded: 1},
			wantStatus: model.DingTalkNotificationStatusSucceeded,
		},
		{
			name: "canceled during failed send", sendError: context.Canceled,
			wantResult: announcementGroupDeliveryResult{Attempted: 1, Failed: 1},
			wantStatus: model.DingTalkNotificationStatusFailed,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			setupAnnouncementGroupNotificationTest(t)
			records := []model.DingTalkNotification{
				{
					EventType: model.DingTalkNotificationEventAnnouncementGroup,
					DedupeKey: "announcement_group:first", Recipient: "cid-announcement",
					Title: "First announcement", Content: "First content",
					Status: model.DingTalkNotificationStatusPending, ScheduledAt: 100,
				},
				{
					EventType: model.DingTalkNotificationEventAnnouncementGroup,
					DedupeKey: "announcement_group:second", Recipient: "cid-announcement",
					Title: "Second announcement", Content: "Second content",
					Status: model.DingTalkNotificationStatusPending, ScheduledAt: 100,
				},
			}
			require.NoError(t, model.DB.Create(&records).Error)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			if testCase.canceledBefore {
				cancel()
			}
			previousSend := sendAnnouncementGroupMessage
			calls := 0
			sendAnnouncementGroupMessage = func(sendContext context.Context, _ system_setting.DingTalkSettings, _ string, title, _ string) error {
				calls++
				assert.NoError(t, sendContext.Err())
				assert.Equal(t, records[0].Title, title)
				cancel()
				return testCase.sendError
			}
			t.Cleanup(func() { sendAnnouncementGroupMessage = previousSend })

			result, err := deliverDueAnnouncementGroupNotifications(ctx, 100)

			require.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, testCase.wantResult, result)
			assert.Equal(t, testCase.wantResult.Attempted, calls)
			require.NoError(t, model.DB.First(&records[0], records[0].Id).Error)
			assert.Equal(t, testCase.wantStatus, records[0].Status)
			if testCase.sendError != nil {
				assert.Equal(t, testCase.sendError.Error(), records[0].Error)
			} else {
				assert.Empty(t, records[0].Error)
			}
			require.NoError(t, model.DB.First(&records[1], records[1].Id).Error)
			assert.Equal(t, model.DingTalkNotificationStatusPending, records[1].Status)
			assert.Empty(t, records[1].Error)
			assert.Zero(t, records[1].SentAt)
		})
	}
}

func TestSendDingTalkAnnouncementGroupTestMessageUsesConfiguredGroup(t *testing.T) {
	setupAnnouncementGroupNotificationTest(t)
	previousSend := sendAnnouncementGroupMessage
	called := false
	sendAnnouncementGroupMessage = func(_ context.Context, _ system_setting.DingTalkSettings, conversationID, title, content string) error {
		called = true
		assert.Equal(t, "cid-announcement", conversationID)
		assert.Equal(t, "系统公告测试", title)
		assert.Contains(t, content, "公告群通知配置成功")
		assert.Contains(t, content, "+08:00")
		return nil
	}
	t.Cleanup(func() {
		sendAnnouncementGroupMessage = previousSend
	})

	err := SendDingTalkAnnouncementGroupTestMessage(context.Background())

	require.NoError(t, err)
	assert.True(t, called)
}

func TestDeliverDueAnnouncementGroupNotificationDoesNotFallbackToOldGroup(t *testing.T) {
	setupAnnouncementGroupNotificationTest(t)
	record := &model.DingTalkNotification{
		EventType: model.DingTalkNotificationEventAnnouncementGroup,
		DedupeKey: "announcement_group:cleared", Recipient: "cid-old",
		Title: "系统公告", Content: "公告正文", ScheduledAt: 100,
	}
	_, err := model.UpsertPendingAnnouncementGroupNotification(record)
	require.NoError(t, err)
	system_setting.GetDingTalkSettings().AnnouncementGroupOpenConversationId = ""
	previousSend := sendAnnouncementGroupMessage
	called := false
	sendAnnouncementGroupMessage = func(context.Context, system_setting.DingTalkSettings, string, string, string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { sendAnnouncementGroupMessage = previousSend })

	result, err := deliverDueAnnouncementGroupNotifications(context.Background(), 100)

	require.NoError(t, err)
	assert.False(t, called)
	assert.Equal(t, 1, result.Failed)
	require.NoError(t, model.DB.Where("dedupe_key = ?", record.DedupeKey).First(record).Error)
	assert.Equal(t, model.DingTalkNotificationStatusFailed, record.Status)
	assert.Equal(t, ErrDingTalkGroupNotConfigured.Error(), record.Error)
}
