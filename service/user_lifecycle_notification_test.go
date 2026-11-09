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

func TestNotifyNewUserRegisteredCreatesWelcomeNotification(t *testing.T) {
	db := setupDingTalkNotificationServiceDB(t)
	user := model.User{
		Username: "registered-user", Email: "registered-user@example.com",
		Password: "password", AffCode: "registered-user-aff", Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&user).Error)

	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	*settings = system_setting.DingTalkSettings{}
	t.Cleanup(func() { *settings = previousSettings })

	NotifyNewUserRegistered(user.Id)

	var record model.DingTalkNotification
	require.Eventually(t, func() bool {
		err := db.Where("event_type = ? AND user_id = ?", model.DingTalkNotificationEventUserRegistered, user.Id).
			First(&record).Error
		return err == nil && record.Status == model.DingTalkNotificationStatusSkipped
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, "欢迎使用 iCode", record.Title)
	assert.Equal(t, "请联系部门额度池管理员添加部门额度池，新用户额度池仅供体验，用完即止。", record.Content)
	assert.Equal(t, model.DingTalkNotificationStatusSkipped, record.Status)
}

func TestNotifyQuotaPoolJoinedCreatesNotificationForNormalPool(t *testing.T) {
	db := setupDingTalkNotificationServiceDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaPool{}))
	pool := model.QuotaPool{Name: "研发额度池", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	user := model.User{
		Username: "pool-member", Email: "pool-member@example.com", QuotaPoolId: pool.Id,
		Password: "password", AffCode: "pool-member-aff", Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&user).Error)

	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	*settings = system_setting.DingTalkSettings{}
	t.Cleanup(func() { *settings = previousSettings })

	NotifyQuotaPoolJoined(user.Id, pool.Id)

	var record model.DingTalkNotification
	require.Eventually(t, func() bool {
		err := db.Where("event_type = ? AND user_id = ?", model.DingTalkNotificationEventQuotaPoolJoined, user.Id).
			First(&record).Error
		return err == nil && record.Status == model.DingTalkNotificationStatusSkipped
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, "额度池加入通知", record.Title)
	assert.Equal(t, "您已加入额度池：研发额度池。", record.Content)
	assert.Equal(t, model.DingTalkNotificationStatusSkipped, record.Status)
}
