package model

import (
	"fmt"
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
