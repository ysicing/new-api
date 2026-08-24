package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestListDingTalkNotificationsReturnsFilteredPage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:dingtalk-notification-controller-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.DingTalkNotification{}))
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
	require.NoError(t, db.Create(&[]model.DingTalkNotification{
		{EventType: model.DingTalkNotificationEventNewUserQuotaExhausted, DedupeKey: "event:alice", UserId: 1, Username: "alice", Recipient: "alice", Status: model.DingTalkNotificationStatusSucceeded, CreatedAt: 100},
		{EventType: "quota_recharged", DedupeKey: "event:bob", UserId: 2, Username: "bob", Recipient: "bob", Status: model.DingTalkNotificationStatusFailed, CreatedAt: 200},
	}).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/dingtalk-notifications?event_type=quota_recharged&status=failed&keyword=bob&p=1&page_size=10", nil)

	ListDingTalkNotifications(context)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Page     int                          `json:"page"`
			PageSize int                          `json:"page_size"`
			Total    int                          `json:"total"`
			Items    []model.DingTalkNotification `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Data.Page)
	assert.Equal(t, 10, response.Data.PageSize)
	assert.Equal(t, 1, response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, "event:bob", response.Data.Items[0].DedupeKey)
}
