package reasoning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekModifiersRejectInvalidOrAmbiguousValues(t *testing.T) {
	for _, model := range []string{
		"deepseek-flash@", "deepseek-flash@unknown:on",
		"deepseek-flash@thinking:on@thinking:off",
		"deepseek-flash@thinking:on@effort:max",
		"deepseek-flash@thinking:-2", "deepseek-flash@thinking:banana",
		"deepseek-flash@effort:ultra", "deepseek-flash@temperature:NaN",
		"deepseek-flash@temperature:+Inf", "deepseek-flash@temperature:-0.1",
		"deepseek-flash@temperature:2.1", "@effort:low",
	} {
		t.Run(model, func(t *testing.T) {
			_, err := ParseDeepSeekModifiers(model)
			require.Error(t, err)
		})
	}
}

func TestDeepSeekModifiersPreserveExplicitZeroTemperatureAndBudgetBoundaries(t *testing.T) {
	for _, test := range []struct{ budget, thinking, effort string }{
		{"0", "disabled", ""}, {"1", "enabled", "low"}, {"1024", "enabled", "low"},
		{"1025", "enabled", "medium"}, {"8192", "enabled", "medium"},
		{"8193", "enabled", "high"}, {"-1", "enabled", "high"},
	} {
		t.Run(test.budget, func(t *testing.T) {
			modifiers, err := ParseDeepSeekModifiers("deepseek-flash@thinking:" + test.budget + "@temperature:0")
			require.NoError(t, err)
			assert.Equal(t, "deepseek-flash", modifiers.Model)
			assert.Equal(t, test.thinking, modifiers.Thinking)
			assert.Equal(t, test.effort, modifiers.Effort)
			require.NotNil(t, modifiers.Temperature)
			assert.Zero(t, *modifiers.Temperature)
		})
	}
}
