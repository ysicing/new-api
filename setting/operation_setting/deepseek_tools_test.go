package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekToolDefaultsPreserveConfiguredPricesAndExplicitZero(t *testing.T) {
	saved, err := common.Marshal(toolPriceSetting.Prices)
	require.NoError(t, err)
	t.Cleanup(func() { LoadToolPricesFromJSONString(string(saved)) })
	LoadToolPricesFromJSONString(`{"web_search":10}`)
	assert.Equal(t, 0.75, GetToolPriceForModel("deepseek_search", "deepseek-flash"))
	assert.Equal(t, 0.75, GetToolPriceForModel("deepseek_fetch", "custom-alias"))
	LoadToolPricesFromJSONString(`{"deepseek_search":1.5,"deepseek_fetch":0,"deepseek_search:deepseek-v4-pro*":2}`)
	assert.Equal(t, 1.5, GetToolPriceForModel("deepseek_search", "deepseek-flash"))
	assert.Zero(t, GetToolPriceForModel("deepseek_fetch", "deepseek-v4-pro"))
	assert.Equal(t, 2.0, GetToolPriceForModel("deepseek_search", "deepseek-v4-pro-max"))
}
