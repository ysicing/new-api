package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekCanonicalBillingAndExplicitOverrides(t *testing.T) {
	gin.SetMode(gin.TestMode)
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error { saved[key] = value; return nil }))
	prices, ratios := ratio_setting.ModelPrice2JSONString(), ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(prices))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(ratios))
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{}`, "billing_setting.billing_expr": `{}`,
	}))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"deepseek-v4-pro-low":0.01,"deepseek-flash@effort:low":0.02}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
	for _, test := range []struct {
		model, billing string
		price          float64
	}{
		{"deepseek-flash@effort:max", "deepseek-flash", 0},
		{"deepseek-v4-pro-none", "deepseek-v4-pro", 0},
		{"deepseek-v4-pro-low", "deepseek-v4-pro-low", 0.01},
		{"deepseek-flash@effort:low", "deepseek-flash@effort:low", 0.02},
	} {
		t.Run(test.model, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{OriginModelName: test.model, UserGroup: "default", UsingGroup: "default",
				BillingRequestInput: &billingexpr.RequestInput{}}
			price, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{MaxTokens: 100})
			require.NoError(t, err)
			assert.Equal(t, test.billing, info.GetBillingModelName())
			assert.Equal(t, test.price > 0, price.UsePrice)
			assert.Equal(t, test.price, price.ModelPrice)
			if !price.UsePrice {
				require.NotNil(t, info.TieredBillingSnapshot)
			}
		})
	}
}

func TestDeepSeekBlacklistedModelDoesNotAliasBilling(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	original := settings.ThinkingModelBlacklist
	settings.ThinkingModelBlacklist = []string{"deepseek-v4-pro-max"}
	t.Cleanup(func() { settings.ThinkingModelBlacklist = original })
	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-v4-pro-max", UserGroup: "default", UsingGroup: "default"}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.Error(t, err)
	assert.Equal(t, "deepseek-v4-pro-max", info.GetBillingModelName())
}

func TestDeepSeekStandaloneToolPricesRespectExplicitZeroAndOverrides(t *testing.T) {
	original := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(original)) })
	for _, test := range []struct {
		prices        string
		search, fetch float64
	}{
		{`{}`, 0.00075, 0.00075},
		{`{"deepseek-search":0,"deepseek-fetch":0.002}`, 0, 0.002},
	} {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(test.prices))
		search, ok := ratio_setting.GetModelPrice("deepseek-search", false)
		require.True(t, ok)
		assert.Equal(t, test.search, search)
		fetch, ok := ratio_setting.GetModelPrice("deepseek-fetch", false)
		require.True(t, ok)
		assert.Equal(t, test.fetch, fetch)
	}
}
