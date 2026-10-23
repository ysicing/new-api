package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStatusExposesOnlyPublicDingTalkLoginConfig(t *testing.T) {
	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	previousOptionMap := common.OptionMap
	*settings = system_setting.DingTalkSettings{
		Enabled: true, CorpId: "corp-1", ClientId: "app-key", ClientSecret: "app-secret",
	}
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		*settings = previousSettings
		common.OptionMap = previousOptionMap
	})

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	GetStatus(context)

	var payload struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, true, payload.Data["dingtalk_login"])
	assert.Equal(t, "app-key", payload.Data["dingtalk_client_id"])
	assert.NotContains(t, payload.Data, "dingtalk_client_secret")
	assert.NotContains(t, payload.Data, "dingtalk_corp_id")
}

func TestUpdateOptionRequiresCompleteDingTalkConfigBeforeEnable(t *testing.T) {
	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	*settings = system_setting.DingTalkSettings{ClientId: "app-key"}
	t.Cleanup(func() { *settings = previousSettings })

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/option/", strings.NewReader(`{"key":"dingtalk.enabled","value":true}`))
	UpdateOption(context)

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	assert.Contains(t, payload.Message, "钉钉")
}
