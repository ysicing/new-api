package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func LoginWithLDAP(username, password string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, errors.New("ldap username or password is empty")
	}
	config, err := buildLDAPConfig()
	if err != nil {
		return nil, err
	}
	authUsername := username
	if strings.Contains(authUsername, "@") {
		if prefix := strings.TrimSpace(strings.SplitN(authUsername, "@", 2)[0]); prefix != "" {
			authUsername = prefix
		}
	}
	result, err := ldapRequest(config, authUsername, password)
	if err != nil {
		common.SysLog("LDAP authentication failed for " + username)
		return nil, errors.New("ldap authentication failed")
	}
	profile, err := profileFromSearchResult(result, config.LdapUID, username)
	if err != nil {
		return nil, err
	}
	user, created, err := findOrCreateLDAPUser(profile, common.RegisterEnabled)
	if err != nil {
		return nil, err
	}
	if created {
		NotifyNewUserRegistered(user.Id)
	}
	return user, nil
}
