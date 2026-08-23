package service

import (
	"github.com/QuantumNous/new-api/common"
)

type QuotaPoolCapabilities struct {
	CanView              bool `json:"can_view"`
	CanEdit              bool `json:"can_edit"`
	CanEditMonthlyRefill bool `json:"can_edit_monthly_refill"`
	CanRefill            bool `json:"can_refill"`
	CanManageMembers     bool `json:"can_manage_members"`
	CanRemoveMembers     bool `json:"can_remove_members"`
	CanManageAdmins      bool `json:"can_manage_admins"`
	CanDelete            bool `json:"can_delete"`
}

func ResolveQuotaPoolCapabilities(role int, poolAdmin bool) QuotaPoolCapabilities {
	switch role {
	case common.RoleRootUser:
		return QuotaPoolCapabilities{CanView: true, CanEdit: true, CanEditMonthlyRefill: true, CanRefill: true, CanManageMembers: true, CanRemoveMembers: true, CanManageAdmins: true, CanDelete: true}
	case common.RoleAdminUser:
		return QuotaPoolCapabilities{CanView: true, CanRefill: true, CanManageMembers: true, CanRemoveMembers: true, CanManageAdmins: true}
	case common.RoleQuotaPoolSuperAdmin:
		return QuotaPoolCapabilities{CanView: true, CanEdit: true, CanManageMembers: true, CanRemoveMembers: true, CanManageAdmins: true}
	}
	if poolAdmin {
		return QuotaPoolCapabilities{CanView: true, CanEdit: true, CanManageMembers: true, CanRemoveMembers: true}
	}
	return QuotaPoolCapabilities{}
}
