package controller

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type ldapSyncRequest struct {
	Username string                    `json:"username"`
	Action   string                    `json:"action"`
	User     service.LDAPSyncCandidate `json:"user"`
}

func LDAPLogin(c *gin.Context) {
	var request LoginRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, err := service.LoginWithLDAP(strings.TrimSpace(request.Username), request.Password)
	if err != nil {
		common.ApiErrorMsg(c, "LDAP 用户名或密码错误")
		return
	}
	completePrimaryLogin(user, c)
}

func SyncLDAPUser(c *gin.Context) {
	var request ldapSyncRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	var user *model.User
	var err error
	switch strings.ToLower(strings.TrimSpace(request.Action)) {
	case "search":
		candidates, searchErr := service.SearchLDAPUsers(request.Username)
		if searchErr != nil {
			common.ApiErrorMsg(c, "LDAP 用户查询失败")
			return
		}
		common.ApiSuccess(c, gin.H{"users": candidates, "total": len(candidates)})
		return
	case "sync":
		user, err = service.SyncLDAPCandidate(request.User)
	default:
		user, err = service.SyncLDAPUser(request.Username)
	}
	if err != nil {
		common.ApiErrorMsg(c, "LDAP 用户同步失败")
		return
	}
	recordManageAuditFor(c, user.Id, "user.ldap_sync", map[string]interface{}{
		"identity": ldapAuditIdentity(user),
	})
	common.ApiSuccess(c, gin.H{
		"id": user.Id, "username": user.Username, "display_name": user.DisplayName,
		"department": user.Department, "ldap_id": user.LDAPId, "email": user.Email,
		"role": user.Role, "status": user.Status,
	})
}

func ldapAuditIdentity(user *model.User) string {
	if strings.TrimSpace(user.Email) != "" {
		return user.Email
	}
	if strings.TrimSpace(user.LDAPId) != "" {
		return user.LDAPId
	}
	return fmt.Sprint(user.Id)
}
