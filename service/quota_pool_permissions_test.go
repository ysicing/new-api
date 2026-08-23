package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
)

func TestQuotaPoolCapabilitiesByRoleAndPoolAdminStatus(t *testing.T) {
	tests := []struct {
		name      string
		role      int
		poolAdmin bool
		want      QuotaPoolCapabilities
	}{
		{
			name: "root owns every operation", role: common.RoleRootUser,
			want: QuotaPoolCapabilities{CanView: true, CanEdit: true, CanEditMonthlyRefill: true, CanRefill: true, CanManageMembers: true, CanRemoveMembers: true, CanManageAdmins: true, CanDelete: true},
		},
		{
			name: "system admin manages funds and members", role: common.RoleAdminUser,
			want: QuotaPoolCapabilities{CanView: true, CanRefill: true, CanManageMembers: true, CanRemoveMembers: true, CanManageAdmins: true},
		},
		{
			name: "pool super admin edits policy and manages members", role: common.RoleQuotaPoolSuperAdmin,
			want: QuotaPoolCapabilities{CanView: true, CanEdit: true, CanManageMembers: true, CanRemoveMembers: true, CanManageAdmins: true},
		},
		{
			name: "pool admin manages ordinary members", role: common.RoleCommonUser, poolAdmin: true,
			want: QuotaPoolCapabilities{CanView: true, CanEdit: true, CanManageMembers: true, CanRemoveMembers: true},
		},
		{name: "ordinary user has no management capability", role: common.RoleCommonUser},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ResolveQuotaPoolCapabilities(tt.role, tt.poolAdmin))
		})
	}
}
