package controller

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type ldapSyncRequest struct {
	Username string                    `json:"username"`
	Action   string                    `json:"action"`
	User     service.LDAPSyncCandidate `json:"user"`
}

var authenticateLDAPUser = service.LoginWithLDAP

func LDAPLogin(c *gin.Context) {
	var request LoginRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	password := request.Password
	if common.PasswordLoginEncryptionEnabled {
		if request.PasswordEncrypted == "" || request.EncryptionKeyID == "" {
			common.ApiErrorT(c, "Invalid parameters")
			return
		}
		decrypted, err := common.DecryptPassword(request.PasswordEncrypted, request.EncryptionKeyID)
		if err != nil {
			common.ApiErrorT(c, "Username or password is incorrect, or user has been banned")
			return
		}
		password = decrypted
	}
	user, err := authenticateLDAPUser(strings.TrimSpace(request.Username), password)
	if err != nil {
		common.ApiErrorT(c, "Username or password is incorrect, or user has been banned")
		return
	}
	c.Set("login_method", "ldap")
	setupLogin(user, nil, c)
}

func SyncLDAPUser(c *gin.Context) {
	var request ldapSyncRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	var user *model.User
	var err error
	switch strings.ToLower(strings.TrimSpace(request.Action)) {
	case "search":
		candidates, searchErr := service.SearchLDAPUsers(request.Username)
		if searchErr != nil {
			common.ApiErrorT(c, "Failed to search LDAP users")
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
		common.ApiErrorT(c, "Failed to synchronize LDAP user")
		return
	}
	recordManageAuditFor(c, user.Id, "user.ldap_sync", map[string]any{
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
