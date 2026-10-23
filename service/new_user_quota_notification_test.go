package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupNewUserQuotaNotificationTest(t *testing.T) (*model.QuotaPool, *model.QuotaPool) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:new-user-quota-notification-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.QuotaPool{}, &model.DingTalkNotification{}))
	previousDB, previousRedis := model.DB, common.RedisEnabled
	model.DB, common.RedisEnabled = db, false
	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	*settings = system_setting.DingTalkSettings{}
	t.Cleanup(func() {
		model.DB, common.RedisEnabled = previousDB, previousRedis
		*settings = previousSettings
	})
	newUserPool := &model.QuotaPool{Name: model.QuotaPoolNewUserName, PoolType: model.QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}
	normalPool := &model.QuotaPool{Name: "研发池", PoolType: model.QuotaPoolTypeNormal, Enabled: true, BaseQuota: 100, Quota: 100}
	require.NoError(t, db.Create(newUserPool).Error)
	require.NoError(t, db.Create(normalPool).Error)
	return newUserPool, normalPool
}

func TestNotifyNewUserQuotaExhaustedCreatesOneOperationsRecord(t *testing.T) {
	newUserPool, _ := setupNewUserQuotaNotificationTest(t)
	user := model.User{Username: "alice", Email: "alice@example.com", Password: "password", AffCode: "new-user-notify", QuotaPoolId: newUserPool.Id, Quota: 0}
	require.NoError(t, model.DB.Create(&user).Error)
	relayInfo := &relaycommon.RelayInfo{UserId: user.Id, UserEmail: user.Email, UserQuota: 20}

	require.NoError(t, notifyNewUserQuotaExhausted(relayInfo, 20))
	require.NoError(t, notifyNewUserQuotaExhausted(relayInfo, 20))

	var records []model.DingTalkNotification
	require.NoError(t, model.DB.Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, model.DingTalkNotificationEventNewUserQuotaExhausted, records[0].EventType)
	assert.Equal(t, "new_user_quota_exhausted:"+fmt.Sprint(user.Id), records[0].DedupeKey)
	assert.Equal(t, "体验额度已用完", records[0].Title)
	assert.Equal(t, "当前体验额度已经用完，请联系部门额度池管理员及时添加到对应额度池。", records[0].Content)
	assert.Equal(t, model.DingTalkNotificationStatusSkipped, records[0].Status)
	assert.Contains(t, records[0].Metadata, `"quota_before":20`)
	assert.Contains(t, records[0].Metadata, `"quota_after":0`)
}

func TestNotifyNewUserQuotaExhaustedIgnoresOtherPoolsAndNonTransitions(t *testing.T) {
	newUserPool, normalPool := setupNewUserQuotaNotificationTest(t)
	users := []model.User{
		{Username: "normal", Email: "normal@example.com", Password: "password", AffCode: "normal-pool-notify", QuotaPoolId: normalPool.Id},
		{Username: "already-zero", Email: "zero@example.com", Password: "password", AffCode: "zero-pool-notify", QuotaPoolId: newUserPool.Id},
		{Username: "subscription", Email: "subscription@example.com", Password: "password", AffCode: "subscription-pool-notify", QuotaPoolId: newUserPool.Id},
		{Username: "recharged", Email: "recharged@example.com", Password: "password", AffCode: "recharged-pool-notify", QuotaPoolId: newUserPool.Id, Quota: 100},
	}
	require.NoError(t, model.DB.Create(&users).Error)

	require.NoError(t, notifyNewUserQuotaExhausted(&relaycommon.RelayInfo{UserId: users[0].Id, UserQuota: 20}, 20))
	require.NoError(t, notifyNewUserQuotaExhausted(&relaycommon.RelayInfo{UserId: users[1].Id, UserQuota: 0}, 20))
	require.NoError(t, notifyNewUserQuotaExhausted(&relaycommon.RelayInfo{UserId: users[2].Id, UserQuota: 20, BillingSource: BillingSourceSubscription}, 20))
	require.NoError(t, notifyNewUserQuotaExhausted(&relaycommon.RelayInfo{UserId: users[3].Id, UserQuota: 20}, 20))

	var count int64
	require.NoError(t, model.DB.Model(&model.DingTalkNotification{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestPostConsumeQuotaNotifiesNewUserPoolWhenEmailAlertIsDisabled(t *testing.T) {
	newUserPool, _ := setupNewUserQuotaNotificationTest(t)
	user := model.User{Username: "realtime", Email: "realtime@example.com", Password: "password", AffCode: "realtime-notify", QuotaPoolId: newUserPool.Id, Quota: 20}
	require.NoError(t, model.DB.Create(&user).Error)
	relayInfo := &relaycommon.RelayInfo{
		UserId: user.Id, UserEmail: user.Email, UserQuota: 20,
		IsPlayground: true,
	}

	require.NoError(t, PostConsumeQuota(relayInfo, 20, 0, false))

	require.Eventually(t, func() bool {
		var count int64
		return model.DB.Model(&model.DingTalkNotification{}).Where("user_id = ?", user.Id).Count(&count).Error == nil && count == 1
	}, time.Second, 10*time.Millisecond)
}
