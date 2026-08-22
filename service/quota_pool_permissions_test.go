package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestQuotaPoolCapabilitiesByRoleAndPoolAdminLevel(t *testing.T) {
	tests := []struct {
		name  string
		role  int
		level int
		want  QuotaPoolCapabilities
	}{
		{
			name: "root owns every operation", role: common.RoleRootUser,
			want: QuotaPoolCapabilities{CanView: true, CanEdit: true, CanEditMonthlyRefill: true, CanRefill: true, CanManageMembers: true, CanManageV1Admins: true, CanManageV2Admins: true, CanDelete: true},
		},
		{
			name: "system admin manages funds and members", role: common.RoleAdminUser,
			want: QuotaPoolCapabilities{CanView: true, CanRefill: true, CanManageMembers: true, CanManageV1Admins: true, CanManageV2Admins: true},
		},
		{
			name: "pool super admin edits policy and manages members", role: common.RoleQuotaPoolSuperAdmin,
			want: QuotaPoolCapabilities{CanView: true, CanEdit: true, CanManageMembers: true, CanManageV1Admins: true, CanManageV2Admins: true},
		},
		{
			name: "pool v2 can manage v1", role: common.RoleCommonUser, level: model.QuotaPoolAdminLevelV2,
			want: QuotaPoolCapabilities{CanView: true, CanEdit: true, CanManageMembers: true, CanManageV1Admins: true},
		},
		{
			name: "pool v1 manages members", role: common.RoleCommonUser, level: model.QuotaPoolAdminLevelV1,
			want: QuotaPoolCapabilities{CanView: true, CanEdit: true, CanManageMembers: true},
		},
		{name: "ordinary user has no management capability", role: common.RoleCommonUser},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ResolveQuotaPoolCapabilities(tt.role, tt.level))
		})
	}
}
