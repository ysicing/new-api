package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionSynchronizesAnnouncementGroupNotificationsAfterSave(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.Log{}))
	previousAnnouncements := console_setting.GetConsoleSetting().Announcements
	previousOptionMap := common.OptionMap
	oldValue := `[{"id":1,"content":"旧公告","publishDate":"2026-09-01T08:00:00+08:00","type":"default"}]`
	newValue := `[{"id":1,"content":"旧公告","publishDate":"2026-09-01T08:00:00+08:00","type":"default"},{"id":2,"content":"新公告","publishDate":"2026-09-01T12:00:00+08:00","type":"warning"}]`
	console_setting.GetConsoleSetting().Announcements = oldValue
	common.OptionMap = map[string]string{"console_setting.announcements": oldValue}
	previousSync := syncAnnouncementGroupNotifications
	called := false
	syncAnnouncementGroupNotifications = func(previous, current string, operatorId int, operatorName string, _ time.Time) error {
		called = true
		assert.Equal(t, oldValue, previous)
		assert.Equal(t, newValue, current)
		assert.Equal(t, 9, operatorId)
		assert.Equal(t, "root", operatorName)
		return nil
	}
	t.Cleanup(func() {
		console_setting.GetConsoleSetting().Announcements = previousAnnouncements
		common.OptionMap = previousOptionMap
		syncAnnouncementGroupNotifications = previousSync
	})
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/option/", strings.NewReader(`{"key":"console_setting.announcements","value":`+strconvQuote(newValue)+`}`))
	context.Set("id", 9)
	context.Set("username", "root")

	UpdateOption(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, called)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
}

func strconvQuote(value string) string {
	data, _ := common.Marshal(value)
	return string(data)
}
