package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuditContentENFormatsUserQuotaPoolRecharge(t *testing.T) {
	content := auditContentEN("user.quota_pool_recharge", map[string]interface{}{
		"username":       "alice",
		"target_user_id": 25,
		"quota":          "¥10.000000 额度",
	})

	assert.Equal(t, "Replenished quota for user alice (ID: 25): ¥10.000000 额度", content)
}
