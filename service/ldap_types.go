package service

import (
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"

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
	if username == "" || password == "" {
		return nil, errors.New("ldap credentials are empty")
	}
	connection, err := connectLDAP(config)
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	result, err := searchLDAPDirectory(connection, config, username, false)
	if err != nil {
		return nil, err
	}
	if len(result.Entries) != 1 || result.Entries[0].DN == "" {
		return nil, errors.New("ldap identity is ambiguous")
	}
	if err := connection.Bind(result.Entries[0].DN, password); err != nil {
		return nil, errors.New("ldap authentication failed")
	}
	return result, nil
}

var ldapSearch = func(config ergoldap.LdapConf, username string) []ergoldap.LdapUser {
	connection, err := connectLDAP(config)
	if err != nil {
		return nil
	}
	defer connection.Close()
	result, err := searchLDAPDirectory(connection, config, username, true)
	if err != nil {
		return nil
	}
	users := make([]ergoldap.LdapUser, 0, len(result.Entries))
	for _, entry := range result.Entries {
		profile, err := profileFromSearchResult(&goldap.SearchResult{Entries: []*goldap.Entry{entry}}, config.LdapUID, username)
		if err != nil {
			return nil
		}
		users = append(users, ergoldap.LdapUser{
			Username: profile.Username, Email: profile.Email, DisplayName: profile.DisplayName, Department: profile.Department, DN: entry.DN,
		})
	}
	return users
}

// connectLDAP 在发送任何绑定密码前验证服务端证书；ldap:// 必须成功升级 StartTLS。
func connectLDAP(config ergoldap.LdapConf) (*goldap.Conn, error) {
	address := config.LdapURL
	if !strings.Contains(address, "://") {
		address = "ldap://" + address
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "ldap" && parsed.Scheme != "ldaps") || parsed.User != nil {
		return nil, errors.New("ldap URL is invalid")
	}
	timeout := time.Duration(config.LdapConnectionTimeout) * time.Second
	tlsConfig := &tls.Config{ServerName: parsed.Hostname(), MinVersion: tls.VersionTLS12}
	connection, err := goldap.DialURL(address, goldap.DialWithDialer(&net.Dialer{Timeout: timeout}), goldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return nil, err
	}
	connection.SetTimeout(timeout)
	if parsed.Scheme == "ldap" {
		if err := connection.StartTLS(tlsConfig); err != nil {
			connection.Close()
			return nil, err
		}
	}
	if config.LdapSearchDn != "" {
		if config.LdapSearchPassword == "" {
			connection.Close()
			return nil, errors.New("ldap search password is empty")
		}
		if err := connection.Bind(config.LdapSearchDn, config.LdapSearchPassword); err != nil {
			connection.Close()
			return nil, errors.New("ldap search authentication failed")
		}
	}
	return connection, nil
}

func searchLDAPDirectory(connection *goldap.Conn, config ergoldap.LdapConf, username string, prefix bool) (*goldap.SearchResult, error) {
	identifier := goldap.EscapeFilter(username)
	if prefix {
		identifier += "*"
	}
	filter := "(" + config.LdapUID + "=" + identifier + ")"
	if config.LdapFilter != "" {
		filter = "(&" + config.LdapFilter + filter + ")"
	}
	return connection.Search(goldap.NewSearchRequest(
		config.LdapBaseDn, config.LdapScope-1, goldap.NeverDerefAliases, 1000, config.LdapConnectionTimeout, false, filter,
		[]string{config.LdapUID, "uid", "cn", "mail", "email", "displayName", "department"}, nil,
	))
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
