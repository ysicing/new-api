package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

// TestFormatUserLogsStripsQuotaSaturation verifies the admin-only quota
// saturation marker (nested under other.admin_info) is removed for non-admin
// log views, since formatUserLogs strips the whole admin_info object.
func TestFormatUserLogsStripsQuotaSaturation(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.004,
		"admin_info": map[string]interface{}{
			"quota_saturation": map[string]interface{}{
				"op":      "QuotaFromDecimal",
				"kind":    "overflow",
				"clamped": common.MaxQuota,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	_, hasAdminInfo := parsed["admin_info"]
	require.False(t, hasAdminInfo, "admin_info (and nested quota_saturation) must be stripped for non-admin views")
	// Non-admin billing fields remain visible.
	require.Contains(t, parsed, "model_price")
}

func TestFormatUserLogsStripsRequestUserAgentExceptLogin(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"user_agent": "codex-cli/1.2",
	})
	logs := []*Log{
		{Type: LogTypeConsume, Other: other},
		{Type: LogTypeError, Other: other},
		{Type: LogTypeLogin, Other: other},
	}

	formatUserLogs(logs, 0)

	for _, index := range []int{0, 1} {
		parsed, err := common.StrToMap(logs[index].Other)
		require.NoError(t, err)
		require.NotContains(t, parsed, "user_agent")
	}
	parsed, err := common.StrToMap(logs[2].Other)
	require.NoError(t, err)
	require.Equal(t, "codex-cli/1.2", parsed["user_agent"])
}
