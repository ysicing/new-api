package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateDefaultSidebarConfigForRoleIncludesTools(t *testing.T) {
	roles := []int{
		common.RoleCommonUser,
		common.RoleAdminUser,
		common.RoleRootUser,
	}

	for _, role := range roles {
		t.Run("role", func(t *testing.T) {
			var config map[string]map[string]bool
			err := common.Unmarshal(
				[]byte(generateDefaultSidebarConfigForRole(role)),
				&config,
			)

			require.NoError(t, err)
			assert.True(t, config["personal"]["tools"])
		})
	}
}
