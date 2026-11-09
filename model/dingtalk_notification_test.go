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

func verifyDingTalkNotificationSchemaAndDelivery(t *testing.T, mainDB, logDB *gorm.DB) {
	t.Helper()
	existing := DingTalkNotification{
		EventType: DingTalkNotificationEventTest, DedupeKey: "existing:notification",
		Username: "recipient_%", Recipient: "staff", Title: "Existing notification",
		Content: "Retained content", Metadata: `{"source":"rc25"}`,
		Status: DingTalkNotificationStatusSucceeded, SentAt: 123, CreatedAt: 100,
	}
	require.NoError(t, mainDB.Create(&existing).Error)
	require.NoError(t, logDB.Create(&DingTalkNotification{
		EventType: existing.EventType, DedupeKey: "log-only:notification",
		Username: existing.Username, Status: existing.Status, CreatedAt: 100,
	}).Error)
	recorder := &migrationSQLRecorder{}
	migrationDB := mainDB.Session(&gorm.Session{Logger: recorder})
	for range 2 {
		require.NoError(t, migrationDB.AutoMigrate(&DingTalkNotification{}))
	}
	assert.Empty(t, recorder.schemaMutations(), "restarting must preserve the notification schema")
	var stored DingTalkNotification
	require.NoError(t, mainDB.First(&stored, existing.Id).Error)
	assert.Equal(t, existing, stored)
	duplicate := existing
	duplicate.Id = 0
	created, err := CreateDingTalkNotification(&duplicate)
	require.NoError(t, err)
	assert.False(t, created, "the unique event constraint must survive migration")
	created, err = CreateDingTalkNotification(&DingTalkNotification{
		EventType: existing.EventType, DedupeKey: "wildcard-decoy",
		Username: "recipientXY", Status: existing.Status, CreatedAt: 100,
	})
	require.NoError(t, err)
	assert.True(t, created)
	for _, keyword := range []string{"%", "_", "recipient_%"} {
		items, total, err := ListDingTalkNotifications(DingTalkNotificationQuery{
			EventType: existing.EventType, Keyword: keyword,
			StartTimestamp: 100, EndTimestamp: 100, PageSize: 1,
		})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total, "keyword: %s", keyword)
		require.Len(t, items, 1)
		assert.Equal(t, existing.Id, items[0].Id)
	}
	items, total, err := ListDingTalkNotifications(DingTalkNotificationQuery{
		EventType: existing.EventType, StartIdx: 1, PageSize: 1,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, items, 1)
	assert.Equal(t, existing.Id, items[0].Id)

	user := User{Username: "notification-rate-user", AffCode: "notification-rate-user"}
	require.NoError(t, mainDB.Create(&user).Error)
	for index := range 4 {
		notice := DingTalkNotification{
			EventType: DingTalkNotificationEventSensitiveWordDetected,
			DedupeKey: fmt.Sprintf("rate:%d", index), UserId: user.Id,
			Status: DingTalkNotificationStatusPending, CreatedAt: 100,
		}
		created, allowed, err := CreateHourlyRateLimitedDingTalkNotification(&notice, 0, 3600, 3)
		require.NoError(t, err)
		assert.True(t, created)
		assert.Equal(t, index < 3, allowed)
		if index == 3 {
			var skipped DingTalkNotification
			require.NoError(t, mainDB.First(&skipped, notice.Id).Error)
			assert.Equal(t, DingTalkNotificationStatusSkipped, skipped.Status)
			assert.Equal(t, dingTalkNotificationHourlyLimitError, skipped.Error)
		}
		notice.Id = 0
		created, allowed, err = CreateHourlyRateLimitedDingTalkNotification(&notice, 0, 3600, 3)
		require.NoError(t, err)
		assert.False(t, created)
		assert.False(t, allowed)
	}

	announcement := DingTalkNotification{
		EventType: DingTalkNotificationEventAnnouncementGroup, DedupeKey: "announcement:matrix",
		Recipient: "cid-group", Title: "Announcement", Content: "Initial", ScheduledAt: 200,
	}
	created, err = UpsertPendingAnnouncementGroupNotification(&announcement)
	require.NoError(t, err)
	assert.True(t, created)
	announcement.Content, announcement.ScheduledAt = "Updated", 300
	created, err = UpsertPendingAnnouncementGroupNotification(&announcement)
	require.NoError(t, err)
	assert.False(t, created)
	claimed, err := ClaimDueAnnouncementGroupNotifications(299, 10)
	require.NoError(t, err)
	assert.Empty(t, claimed)
	claimed, err = ClaimDueAnnouncementGroupNotifications(300, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, "Updated", claimed[0].Content)
	assert.Equal(t, DingTalkNotificationStatusRunning, claimed[0].Status)
	require.NoError(t, UpdateDingTalkNotificationResult(claimed[0].Id, DingTalkNotificationStatusSucceeded, "", 301))
	claimed, err = ClaimDueAnnouncementGroupNotifications(300, 10)
	require.NoError(t, err)
	assert.Empty(t, claimed)
	var delivered DingTalkNotification
	require.NoError(t, mainDB.Where("dedupe_key = ?", announcement.DedupeKey).First(&delivered).Error)
	assert.Equal(t, DingTalkNotificationStatusSucceeded, delivered.Status)
	assert.EqualValues(t, 301, delivered.SentAt)
	var logCount int64
	require.NoError(t, logDB.Model(&DingTalkNotification{}).Count(&logCount).Error)
	assert.EqualValues(t, 1, logCount, "notifications must stay in the main database")
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
