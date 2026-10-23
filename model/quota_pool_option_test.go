package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionMapUpdatesQuotaPoolEnabled(t *testing.T) {
	previousEnabled := common.QuotaPoolEnabled
	previousMap := common.OptionMap
	t.Cleanup(func() {
		common.QuotaPoolEnabled = previousEnabled
		common.OptionMap = previousMap
	})
	common.QuotaPoolEnabled = false
	common.OptionMap = map[string]string{}

	require.NoError(t, updateOptionMap("QuotaPoolEnabled", "true"))

	assert.True(t, common.QuotaPoolEnabled)
	assert.Equal(t, "true", common.OptionMap["QuotaPoolEnabled"])
}

func TestUpdateOptionMapUpdatesSensitiveWordContactMessage(t *testing.T) {
	previousMessage := setting.SensitiveWordContactMessage
	previousMap := common.OptionMap
	t.Cleanup(func() {
		setting.SensitiveWordContactMessage = previousMessage
		common.OptionMap = previousMap
	})
	common.OptionMap = map[string]string{}

	require.NoError(t, updateOptionMap("SensitiveWordContactMessage", "  如有误判，请联系张三。  "))

	assert.Equal(t, "如有误判，请联系张三。", setting.SensitiveWordContactMessage)
	assert.Equal(t, "如有误判，请联系张三。", common.OptionMap["SensitiveWordContactMessage"])
}
