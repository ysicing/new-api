package system_setting

import "github.com/QuantumNous/new-api/setting/config"

type LDAPSettings struct {
	Enabled           bool   `json:"enabled"`
	URL               string `json:"ldap_url"`
	SearchDN          string `json:"ldap_search_dn"`
	SearchPassword    string `json:"ldap_search_password"`
	BaseDN            string `json:"ldap_base_dn"`
	Filter            string `json:"ldap_filter"`
	UID               string `json:"ldap_uid"`
	Scope             int    `json:"ldap_scope"`
	ConnectionTimeout int    `json:"ldap_connection_timeout"`
}

var defaultLDAPSettings = LDAPSettings{
	UID:               "uid",
	Scope:             3,
	ConnectionTimeout: 30,
}

func init() {
	config.GlobalConfig.Register("ldap", &defaultLDAPSettings)
}

func GetLDAPSettings() *LDAPSettings {
	return &defaultLDAPSettings
}
