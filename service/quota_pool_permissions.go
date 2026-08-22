package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type QuotaPoolCapabilities struct {
	CanView              bool `json:"can_view"`
	CanEdit              bool `json:"can_edit"`
	CanEditMonthlyRefill bool `json:"can_edit_monthly_refill"`
	CanRefill            bool `json:"can_refill"`
	CanManageMembers     bool `json:"can_manage_members"`
	CanManageV1Admins    bool `json:"can_manage_v1_admins"`
	CanManageV2Admins    bool `json:"can_manage_v2_admins"`
	CanDelete            bool `json:"can_delete"`
}

func ResolveQuotaPoolCapabilities(role, poolAdminLevel int) QuotaPoolCapabilities {
	switch role {
	case common.RoleRootUser:
		return QuotaPoolCapabilities{CanView: true, CanEdit: true, CanEditMonthlyRefill: true, CanRefill: true, CanManageMembers: true, CanManageV1Admins: true, CanManageV2Admins: true, CanDelete: true}
	case common.RoleAdminUser:
		return QuotaPoolCapabilities{CanView: true, CanRefill: true, CanManageMembers: true, CanManageV1Admins: true, CanManageV2Admins: true}
	case common.RoleQuotaPoolSuperAdmin:
		return QuotaPoolCapabilities{CanView: true, CanEdit: true, CanManageMembers: true, CanManageV1Admins: true, CanManageV2Admins: true}
	}
	if poolAdminLevel >= model.QuotaPoolAdminLevelV2 {
		return QuotaPoolCapabilities{CanView: true, CanEdit: true, CanManageMembers: true, CanManageV1Admins: true}
	}
	if poolAdminLevel >= model.QuotaPoolAdminLevelV1 {
		return QuotaPoolCapabilities{CanView: true, CanEdit: true, CanManageMembers: true}
	}
	return QuotaPoolCapabilities{}
}
