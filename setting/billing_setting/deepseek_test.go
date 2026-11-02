package billing_setting_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekPeakWindowBoundaries(t *testing.T) {
	expression, ok := billing_setting.GetBillingExpr("deepseek-flash")
	require.True(t, ok)
	for _, test := range []struct {
		weekday, hour int
		tier          string
	}{
		{1, 8, "off_peak"}, {1, 9, "peak"}, {1, 11, "peak"}, {1, 12, "off_peak"},
		{1, 13, "off_peak"}, {1, 14, "peak"}, {5, 17, "peak"}, {5, 18, "off_peak"},
		{6, 10, "off_peak"}, {0, 15, "off_peak"},
	} {
		frozen := strings.NewReplacer(`weekday("Asia/Shanghai")`, strconv.Itoa(test.weekday),
			`hour("Asia/Shanghai")`, strconv.Itoa(test.hour)).Replace(expression)
		_, trace, err := billingexpr.RunExpr(frozen, billingexpr.TokenParams{P: 1000})
		require.NoError(t, err)
		assert.Equal(t, test.tier, trace.MatchedTier)
	}
}

func TestDeepSeekPeakPricingNormalizesCachedTokensAcrossProtocols(t *testing.T) {
	ratios, prices := ratio_setting.ModelRatio2JSONString(), ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(ratios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(prices))
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))
	for _, test := range []struct {
		model         string
		peak, offPeak int
	}{
		{"deepseek-flash", 456, 228},
		{"deepseek-v4-flash", 456, 228},
		{"deepseek-v4-pro", 1694, 847},
	} {
		t.Run(test.model, func(t *testing.T) {
			expression, ok := billing_setting.GetBillingExpr(test.model)
			require.True(t, ok)
			branches := strings.SplitN(expression, " ? ", 2)
			require.Len(t, branches, 2)
			for _, peak := range []bool{true, false} {
				condition, tier, expected := "false", "off_peak", test.offPeak
				if peak {
					condition, tier, expected = "true", "peak", test.peak
				}
				expression := condition + " ? " + branches[1]
				snapshot := &billingexpr.BillingSnapshot{
					ExprString: expression, ExprHash: billingexpr.ExprHashString(expression),
					QuotaPerUnit: 500000, GroupRatio: 1,
				}
				for _, claude := range []bool{true, false} {
					usage := &dto.Usage{PromptTokens: 3000, CompletionTokens: 500}
					if claude {
						usage.PromptTokens = 1000
						usage.UsageSemantic = "anthropic"
					}
					usage.PromptTokensDetails.CachedTokens = 2000
					params := service.BuildTieredTokenParams(usage, claude, map[string]bool{"cr": true})
					result, err := billingexpr.ComputeTieredQuota(snapshot, params)
					require.NoError(t, err)
					assert.Equal(t, expected, result.ActualQuotaAfterGroup)
					assert.Equal(t, tier, result.MatchedTier)
					assert.Nil(t, result.Clamp)
				}
			}
		})
	}
}
