package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
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
