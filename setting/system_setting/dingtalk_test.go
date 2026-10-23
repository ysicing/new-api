package system_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDingTalkSettingsPersistenceRoundTrip(t *testing.T) {
	settings := &DingTalkSettings{
		Enabled: true, CorpId: "corp-1", ClientId: "app-key", ClientSecret: "app-secret",
		AnnouncementGroupOpenConversationId: "cid-announcement",
	}
	manager := config.NewConfigManager()
	manager.Register("dingtalk", settings)

	saved := make(map[string]string)
	require.NoError(t, manager.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	settings.ClientSecret = ""
	require.NoError(t, manager.LoadFromDB(saved))

	assert.Equal(t, "app-secret", settings.ClientSecret)
	assert.Equal(t, "corp-1", settings.CorpId)
	assert.Equal(t, "cid-announcement", settings.AnnouncementGroupOpenConversationId)
	assert.True(t, settings.Enabled)
}

func TestDingTalkRobotConfigurationUsesApplicationCredentials(t *testing.T) {
	assert.True(t, (DingTalkSettings{ClientId: "app-key", ClientSecret: "app-secret"}).IsRobotConfigured())
	assert.False(t, (DingTalkSettings{ClientId: "app-key"}).IsRobotConfigured())
}
