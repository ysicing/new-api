package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	ergoldap "github.com/ergoapi/ldap"
	goldap "github.com/go-ldap/ldap/v3"
)

var (
	ErrLDAPLoginDisabled    = errors.New("ldap login disabled")
	ErrLDAPCandidateInvalid = errors.New("ldap candidate invalid")
)

type ldapProfile struct {
	Username    string
	Email       string
	DisplayName string
	Department  string
	LDAPId      string
}

type LDAPSyncCandidate struct {
	Key         string `json:"key"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Department  string `json:"department"`
	LDAPId      string `json:"ldap_id"`
	Signature   string `json:"signature"`
	ExpiresAt   int64  `json:"expires_at"`
}

var ldapRequest = func(config ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
	return config.LdapReq(username, password)
}

var ldapSearch = func(config ergoldap.LdapConf, username string) []ergoldap.LdapUser {
	return config.Search(username)
}

func buildLDAPConfig() (ergoldap.LdapConf, error) {
	settings := system_setting.GetLDAPSettings()
	if !settings.Enabled {
		return ergoldap.LdapConf{}, ErrLDAPLoginDisabled
	}
	if strings.TrimSpace(settings.URL) == "" || strings.TrimSpace(settings.BaseDN) == "" {
		return ergoldap.LdapConf{}, errors.New("ldap configuration incomplete")
	}
	uid := strings.TrimSpace(settings.UID)
	if uid == "" {
		uid = "uid"
	}
	if strings.ContainsFunc(uid, func(character rune) bool {
		return character != '-' && character != '.' && (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9')
	}) {
		return ergoldap.LdapConf{}, errors.New("ldap user attribute is invalid")
	}
	scope := settings.Scope
	if scope == 0 {
		scope = 3
	}
	if scope < 1 || scope > 3 {
		return ergoldap.LdapConf{}, errors.New("ldap scope is invalid")
	}
	timeout := settings.ConnectionTimeout
	if timeout <= 0 {
		timeout = 30
	}
	timeout = min(timeout, 120)
	if filter := strings.TrimSpace(settings.Filter); filter != "" {
		if _, err := goldap.CompileFilter(filter); err != nil {
			return ergoldap.LdapConf{}, errors.New("ldap search filter is invalid")
		}
	}
	return ergoldap.LdapConf{
		LdapURL: strings.TrimSpace(settings.URL), LdapSearchDn: strings.TrimSpace(settings.SearchDN),
		LdapSearchPassword: settings.SearchPassword, LdapBaseDn: strings.TrimSpace(settings.BaseDN),
		LdapFilter: strings.TrimSpace(settings.Filter), LdapUID: uid,
		LdapScope: scope, LdapConnectionTimeout: timeout,
	}, nil
}

func profileFromSearchResult(result *goldap.SearchResult, uid, fallback string) (ldapProfile, error) {
	if result == nil || len(result.Entries) != 1 {
		return ldapProfile{}, errors.New("ldap identity is ambiguous")
	}
	profile := ldapProfile{Username: strings.TrimSpace(fallback)}
	uid = strings.ToLower(strings.TrimSpace(uid))
	for _, attribute := range result.Entries[0].Attributes {
		if len(attribute.Values) == 0 {
			continue
		}
		name, value := strings.ToLower(attribute.Name), strings.TrimSpace(attribute.Values[0])
		switch {
		case name == "mail" || name == "email":
			profile.Email = model.NormalizeEmail(value)
		case name == uid:
			profile.Username = value
		case name == "displayname":
			profile.DisplayName = value
		case name == "department":
			profile.Department = value
		case name == "cn" && profile.DisplayName == "":
			profile.DisplayName = value
		}
	}
	return normalizeLDAPProfile(profile)
}

func profileFromLDAPUser(user ergoldap.LdapUser, fallback string) (ldapProfile, error) {
	profile := ldapProfile{
		Username: strings.TrimSpace(user.Username), Email: model.NormalizeEmail(user.Email),
		DisplayName: strings.TrimSpace(user.DisplayName), Department: strings.TrimSpace(user.Department),
	}
	if profile.Username == "" {
		profile.Username = strings.TrimSpace(fallback)
	}
	if profile.DisplayName == "" {
		profile.DisplayName = strings.TrimSpace(user.Realname)
	}
	return normalizeLDAPProfile(profile)
}

func normalizeLDAPProfile(profile ldapProfile) (ldapProfile, error) {
	profile.Email = model.NormalizeEmail(profile.Email)
	profile.Username = strings.TrimSpace(profile.Username)
	profile.DisplayName = trimLDAPRunes(strings.TrimSpace(profile.DisplayName), model.UserNameMaxLength)
	profile.Department = trimLDAPRunes(strings.TrimSpace(profile.Department), model.UserDepartmentMaxLength)
	if profile.Email != "" && !strings.Contains(profile.Email, "@") {
		return ldapProfile{}, errors.New("ldap email is invalid")
	}
	if profile.LDAPId == "" {
		profile.LDAPId = profile.Email
		if profile.LDAPId == "" {
			profile.LDAPId = profile.Username
		}
	}
	profile.LDAPId = strings.TrimSpace(profile.LDAPId)
	if profile.LDAPId == "" || len(profile.LDAPId) > 256 {
		return ldapProfile{}, errors.New("ldap identity is invalid")
	}
	if profile.Username == "" {
		profile.Username = profile.LDAPId
	}
	return profile, nil
}

func trimLDAPRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
