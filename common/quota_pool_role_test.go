package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuotaPoolSuperAdminIsValidWithoutBecomingSystemAdmin(t *testing.T) {
	assert.True(t, IsValidateRole(RoleQuotaPoolSuperAdmin))
	assert.Less(t, RoleQuotaPoolSuperAdmin, RoleAdminUser)
}
