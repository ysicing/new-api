package system_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLDAPSettingsUseSafeDefaults(t *testing.T) {
	setting := GetLDAPSettings()

	assert.False(t, setting.Enabled)
	assert.Equal(t, "uid", setting.UID)
	assert.Equal(t, 3, setting.Scope)
	assert.Equal(t, 30, setting.ConnectionTimeout)
}
