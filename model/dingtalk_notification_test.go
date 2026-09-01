package model

import (
	"fmt"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDingTalkNotificationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:dingtalk-notification-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&DingTalkNotification{}))
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })
	return db
}

func TestCreateHourlyRateLimitedDingTalkNotificationCapsConcurrentReservations(t *testing.T) {
	db := setupDingTalkNotificationTestDB(t)
	require.NoError(t, db.AutoMigrate(&User{}))
	user := User{Username: "rate-limit-user", Password: "password", AffCode: "rate-limit-aff"}
	require.NoError(t, db.Create(&user).Error)

	const requestCount = 4
	allowedResults := make(chan bool, requestCount)
	errorResults := make(chan error, requestCount)
	var waitGroup sync.WaitGroup
	for index := 0; index < requestCount; index++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			record := DingTalkNotification{
				EventType: DingTalkNotificationEventSensitiveWordDetected,
				DedupeKey: fmt.Sprintf("sensitive:%d", index), UserId: user.Id,
				Status: DingTalkNotificationStatusPending, CreatedAt: 100,
			}
			_, allowed, err := CreateHourlyRateLimitedDingTalkNotification(&record, 0, 3600, 3)
			allowedResults <- allowed
			errorResults <- err
		}(index)
	}
	waitGroup.Wait()
	close(allowedResults)
	close(errorResults)

	allowedCount := 0
	for allowed := range allowedResults {
		if allowed {
			allowedCount++
		}
	}
	for err := range errorResults {
		require.NoError(t, err)
	}
	assert.Equal(t, 3, allowedCount)
	var skippedCount int64
	require.NoError(t, db.Model(&DingTalkNotification{}).
		Where("status = ?", DingTalkNotificationStatusSkipped).Count(&skippedCount).Error)
	assert.EqualValues(t, 1, skippedCount)
}

func TestCreateDingTalkNotificationDeduplicatesEvent(t *testing.T) {
	db := setupDingTalkNotificationTestDB(t)
	record := &DingTalkNotification{
		EventType: DingTalkNotificationEventNewUserQuotaExhausted,
		DedupeKey: "new_user_quota_exhausted:7",
		UserId:    7, Username: "alice", Recipient: "alice",
		Title: "体验额度已用完", Content: "当前体验额度已经用完",
		Status: DingTalkNotificationStatusPending,
	}

	created, err := CreateDingTalkNotification(record)
	require.NoError(t, err)
	assert.True(t, created)
	assert.NotZero(t, record.Id)

	duplicate := *record
	duplicate.Id = 0
	created, err = CreateDingTalkNotification(&duplicate)
	require.NoError(t, err)
	assert.False(t, created)

	var count int64
	require.NoError(t, db.Model(&DingTalkNotification{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestListDingTalkNotificationsFiltersAndPaginates(t *testing.T) {
	setupDingTalkNotificationTestDB(t)
	records := []DingTalkNotification{
		{EventType: DingTalkNotificationEventNewUserQuotaExhausted, DedupeKey: "event:1", UserId: 1, Username: "alice", Recipient: "alice", Status: DingTalkNotificationStatusSucceeded, CreatedAt: 100},
		{EventType: "quota_recharged", DedupeKey: "event:2", UserId: 2, Username: "bob", Recipient: "bob", Status: DingTalkNotificationStatusFailed, CreatedAt: 200},
		{EventType: DingTalkNotificationEventNewUserQuotaExhausted, DedupeKey: "event:3", UserId: 3, Username: "carol", Recipient: "carol", Status: DingTalkNotificationStatusFailed, CreatedAt: 300},
	}
	require.NoError(t, DB.Create(&records).Error)

	items, total, err := ListDingTalkNotifications(DingTalkNotificationQuery{
		EventType: DingTalkNotificationEventNewUserQuotaExhausted,
		Status:    DingTalkNotificationStatusFailed,
		Keyword:   "carol", StartTimestamp: 250, EndTimestamp: 350,
		StartIdx: 0, PageSize: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "event:3", items[0].DedupeKey)
}

func TestUpdateDingTalkNotificationResultStoresOutcome(t *testing.T) {
	setupDingTalkNotificationTestDB(t)
	record := DingTalkNotification{
		EventType: DingTalkNotificationEventNewUserQuotaExhausted,
		DedupeKey: "event:result", UserId: 9, Status: DingTalkNotificationStatusPending,
	}
	require.NoError(t, DB.Create(&record).Error)

	require.NoError(t, UpdateDingTalkNotificationResult(record.Id, DingTalkNotificationStatusFailed, "invalid recipient", 123))

	var stored DingTalkNotification
	require.NoError(t, DB.First(&stored, record.Id).Error)
	assert.Equal(t, DingTalkNotificationStatusFailed, stored.Status)
	assert.Equal(t, "invalid recipient", stored.Error)
	assert.Equal(t, int64(123), stored.SentAt)
}

func TestAnnouncementGroupNotificationPendingLifecycle(t *testing.T) {
	setupDingTalkNotificationTestDB(t)
	record := &DingTalkNotification{
		EventType: DingTalkNotificationEventAnnouncementGroup,
		DedupeKey: "announcement_group:7", Recipient: "cid-group",
		Title: "系统公告", Content: "初始内容", Status: DingTalkNotificationStatusPending,
		ScheduledAt: 200,
	}

	created, err := UpsertPendingAnnouncementGroupNotification(record)
	require.NoError(t, err)
	assert.True(t, created)
	due, err := ListDueAnnouncementGroupNotifications(199, 10)
	require.NoError(t, err)
	assert.Empty(t, due)

	updated := *record
	updated.Content = "修改后内容"
	updated.ScheduledAt = 300
	created, err = UpsertPendingAnnouncementGroupNotification(&updated)
	require.NoError(t, err)
	assert.False(t, created)
	due, err = ListDueAnnouncementGroupNotifications(300, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, "修改后内容", due[0].Content)

	require.NoError(t, CancelPendingAnnouncementGroupNotification(record.DedupeKey, "announcement deleted before group delivery"))
	due, err = ListDueAnnouncementGroupNotifications(400, 10)
	require.NoError(t, err)
	assert.Empty(t, due)
	var stored DingTalkNotification
	require.NoError(t, DB.Where("dedupe_key = ?", record.DedupeKey).First(&stored).Error)
	assert.Equal(t, DingTalkNotificationStatusSkipped, stored.Status)
	assert.Equal(t, "announcement deleted before group delivery", stored.Error)

	updated.Content = "不能重新打开"
	created, err = UpsertPendingAnnouncementGroupNotification(&updated)
	require.NoError(t, err)
	assert.False(t, created)
	require.NoError(t, DB.Where("dedupe_key = ?", record.DedupeKey).First(&stored).Error)
	assert.Equal(t, DingTalkNotificationStatusSkipped, stored.Status)
}

func TestClaimDueAnnouncementGroupNotificationsMarksRunning(t *testing.T) {
	setupDingTalkNotificationTestDB(t)
	record := &DingTalkNotification{
		EventType: DingTalkNotificationEventAnnouncementGroup,
		DedupeKey: "announcement_group:claim", Recipient: "cid-group",
		Title: "系统公告", Content: "内容", ScheduledAt: 100,
	}
	_, err := UpsertPendingAnnouncementGroupNotification(record)
	require.NoError(t, err)

	claimed, err := ClaimDueAnnouncementGroupNotifications(100, 10)

	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, DingTalkNotificationStatusRunning, claimed[0].Status)
	due, err := ListDueAnnouncementGroupNotifications(100, 10)
	require.NoError(t, err)
	assert.Empty(t, due)
}
