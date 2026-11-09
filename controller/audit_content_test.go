package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuditContentENFormatsUserQuotaPoolRecharge(t *testing.T) {
	content := auditContentEN("user.quota_pool_recharge", map[string]any{
		"username":       "alice",
		"target_user_id": 25,
		"quota":          "¥10.000000 额度",
	})

	assert.Equal(t, "Replenished quota for user alice (ID: 25): ¥10.000000 额度", content)
}

func TestAuditContentENFormatsMagpieActions(t *testing.T) {
	for _, test := range []struct {
		action string
		params map[string]any
		want   string
	}{
		{"channel.magpie_check", map[string]any{"task_id": "task-123"}, "Started Magpie channel check (task ID: task-123)"},
		{"channel.magpie_status", map[string]any{"channel_name": "Codex", "channel_id": 12, "status": 2}, "Updated Magpie channel Codex (ID: 12) to status 2"},
	} {
		t.Run(test.action, func(t *testing.T) {
			assert.Equal(t, test.want, auditContentEN(test.action, test.params))
		})
	}
}
