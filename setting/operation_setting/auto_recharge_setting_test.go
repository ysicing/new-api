package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAutoRechargeDefaultsAreSafeForNewInstallations(t *testing.T) {
	setting := GetAutoRechargeSetting()

	assert.False(t, setting.Enabled)
	assert.Equal(t, 30, setting.Interval)
	assert.Equal(t, 50, setting.Threshold)
	assert.Equal(t, 200, setting.Amount)
	assert.Zero(t, setting.WeeklyLimit)
	assert.Zero(t, setting.MonthlyLimit)
}
