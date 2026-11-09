package setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetSensitiveWordContactMessageTrimsAndRejectsOversizedValue(t *testing.T) {
	previous := SensitiveWordContactMessage
	t.Cleanup(func() { SensitiveWordContactMessage = previous })

	require.NoError(t, SetSensitiveWordContactMessage("  如有误判，请联系张三。  "))
	assert.Equal(t, "如有误判，请联系张三。", SensitiveWordContactMessage)

	err := SetSensitiveWordContactMessage(strings.Repeat("界", SensitiveWordContactMessageMaxRunes+1))
	require.Error(t, err)
	assert.Equal(t, "如有误判，请联系张三。", SensitiveWordContactMessage)
}
