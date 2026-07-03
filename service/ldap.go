package service

import (
	"crypto/hmac"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	ergoldap "github.com/ergoapi/ldap"
	goldap "github.com/go-ldap/ldap/v3"
	"gorm.io/gorm"
)

var ErrLDAPLoginDisabled = errors.New("LDAP 登录未启用")

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
}

type ldapSyncCandidatePayload struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Department  string `json:"department"`
	LDAPId      string `json:"ldap_id"`
}

var ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
	return conf.LdapReq(username, password)
}

var ldapSearch = func(conf ergoldap.LdapConf, username string) []ergoldap.LdapUser {
	return conf.Search(username)
}

func LoginWithLDAP(username, password string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, errors.New("用户名或密码为空")
	}

	settings := system_setting.GetLDAPSettings()
	if !settings.Enabled {
		return nil, ErrLDAPLoginDisabled
	}

	conf, err := buildLDAPConf(settings)
	if err != nil {
		return nil, err
	}

	result, err := authenticateLDAP(conf, username, password)
	if err != nil {
		common.SysLog(fmt.Sprintf("LDAP login failed for %s: %v", username, err))
		return nil, errors.New("LDAP 用户名或密码错误")
	}

	profile, err := ldapProfileFromSearchResult(result, conf.LdapUID, username)
	if err != nil {
		return nil, err
	}

	return findOrCreateLDAPUser(profile, common.RegisterEnabled)
}

func SyncLDAPUser(identifier string) (*model.User, error) {
	profiles, err := searchLDAPProfiles(identifier)
	if err != nil {
		return nil, err
	}
	if len(profiles) != 1 {
		return nil, errors.New("匹配到多个 LDAP 用户，请输入更精确的用户名或邮箱")
	}
	return findOrCreateLDAPUser(profiles[0], true)
}

func SearchLDAPUsers(identifier string) ([]LDAPSyncCandidate, error) {
	profiles, err := searchLDAPProfiles(identifier)
	if err != nil {
		return nil, err
	}

	candidates := make([]LDAPSyncCandidate, 0, len(profiles))
	for _, profile := range profiles {
		candidates = append(candidates, ldapSyncCandidateFromProfile(profile))
	}
	return candidates, nil
}

func searchLDAPProfiles(identifier string) ([]ldapProfile, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, errors.New("LDAP 用户名或邮箱为空")
	}

	conf, err := ldapSyncConf()
	if err != nil {
		return nil, err
	}

	users := searchLDAPUsersForSync(conf, identifier)
	if len(users) == 0 {
		return nil, errors.New("未找到 LDAP 用户")
	}

	profiles := make([]ldapProfile, 0, len(users))
	seen := map[string]struct{}{}
	for _, user := range users {
		profile, err := ldapProfileFromUser(user, identifier)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[profile.Email]; ok {
			continue
		}
		seen[profile.Email] = struct{}{}
		profiles = append(profiles, profile)
	}
	if len(profiles) == 0 {
		return nil, errors.New("未找到 LDAP 用户")
	}
	return profiles, nil
}

func SyncLDAPCandidate(candidate LDAPSyncCandidate) (*model.User, error) {
	if _, err := ldapSyncConf(); err != nil {
		return nil, err
	}
	if !verifyLDAPSyncCandidate(candidate) {
		return nil, errors.New("LDAP 用户信息校验失败，请重新查询后同步")
	}
	profile, err := ldapProfileFromCandidate(candidate)
	if err != nil {
		return nil, err
	}
	return findOrCreateLDAPUser(profile, true)
}

func ldapSyncConf() (ergoldap.LdapConf, error) {
	settings := system_setting.GetLDAPSettings()
	if !settings.Enabled {
		return ergoldap.LdapConf{}, ErrLDAPLoginDisabled
	}
	return buildLDAPConf(settings)
}

func ldapSyncCandidateFromProfile(profile ldapProfile) LDAPSyncCandidate {
	candidate := LDAPSyncCandidate{
		Key:         profile.Email,
		Username:    profile.Username,
		Email:       profile.Email,
		DisplayName: ldapDisplayName(profile),
		Department:  profile.Department,
		LDAPId:      profile.LDAPId,
	}
	candidate.Signature = signLDAPSyncCandidate(candidate)
	return candidate
}

func ldapProfileFromCandidate(candidate LDAPSyncCandidate) (ldapProfile, error) {
	profile := ldapProfile{
		Username:    strings.TrimSpace(candidate.Username),
		Email:       normalizeLDAPEmail(candidate.Email),
		DisplayName: strings.TrimSpace(candidate.DisplayName),
		Department:  strings.TrimSpace(candidate.Department),
		LDAPId:      strings.TrimSpace(candidate.LDAPId),
	}
	if profile.Username == "" {
		profile.Username = profile.Email
	}
	if profile.Email == "" || !strings.Contains(profile.Email, "@") {
		return ldapProfile{}, errors.New("LDAP 用户邮箱为空")
	}
	if len(profile.Email) > 50 {
		return ldapProfile{}, errors.New("LDAP 用户邮箱长度不能超过 50")
	}
	profile.DisplayName = trimRunes(profile.DisplayName, model.UserNameMaxLength)
	profile.Department = trimRunes(profile.Department, model.UserDepartmentMaxLength)
	if profile.LDAPId == "" {
		profile.LDAPId = strings.TrimSpace(ldapBindingId(profile))
	}
	profile.LDAPId = trimRunes(profile.LDAPId, 256)
	return profile, nil
}

func ldapSyncCandidateSignaturePayload(candidate LDAPSyncCandidate) string {
	payload := ldapSyncCandidatePayload{
		Username:    strings.TrimSpace(candidate.Username),
		Email:       normalizeLDAPEmail(candidate.Email),
		DisplayName: strings.TrimSpace(candidate.DisplayName),
		Department:  strings.TrimSpace(candidate.Department),
		LDAPId:      strings.TrimSpace(candidate.LDAPId),
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func signLDAPSyncCandidate(candidate LDAPSyncCandidate) string {
	return common.GenerateHMAC(ldapSyncCandidateSignaturePayload(candidate))
}

func verifyLDAPSyncCandidate(candidate LDAPSyncCandidate) bool {
	signature := strings.TrimSpace(candidate.Signature)
	if signature == "" {
		return false
	}
	expected := signLDAPSyncCandidate(candidate)
	return hmac.Equal([]byte(signature), []byte(expected))
}

func searchLDAPUsersForSync(conf ergoldap.LdapConf, identifier string) []ergoldap.LdapUser {
	users := ldapSearch(conf, identifier)
	if len(users) > 0 || !strings.Contains(identifier, "@") || isLDAPEmailAttribute(conf.LdapUID) {
		return users
	}
	emailConf := conf
	emailConf.LdapUID = "mail"
	return ldapSearch(emailConf, identifier)
}

func authenticateLDAP(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
	username = ldapLoginUsername(username)
	result, err := ldapRequest(conf, username, password)
	if err == nil {
		return result, err
	}

	users := searchLDAPUsersForLogin(conf, username)
	if len(users) != 1 {
		return nil, err
	}

	authIdentifier := strings.TrimSpace(users[0].Username)
	if isLDAPEmailAttribute(conf.LdapUID) {
		authIdentifier = strings.TrimSpace(users[0].Email)
	}
	if authIdentifier == "" {
		return nil, err
	}
	return ldapRequest(conf, authIdentifier, password)
}

func ldapLoginUsername(username string) string {
	username = strings.TrimSpace(username)
	if !strings.Contains(username, "@") {
		return username
	}
	prefix := strings.TrimSpace(strings.SplitN(username, "@", 2)[0])
	if prefix == "" {
		return username
	}
	return prefix
}

func searchLDAPUsersForLogin(conf ergoldap.LdapConf, username string) []ergoldap.LdapUser {
	return ldapSearch(conf, username)
}

func isLDAPEmailAttribute(attr string) bool {
	attr = strings.ToLower(strings.TrimSpace(attr))
	return attr == "mail" || attr == "email"
}

func buildLDAPConf(settings *system_setting.LDAPSettings) (ergoldap.LdapConf, error) {
	if strings.TrimSpace(settings.URL) == "" {
		return ergoldap.LdapConf{}, errors.New("LDAP 地址未配置")
	}
	if strings.TrimSpace(settings.BaseDN) == "" {
		return ergoldap.LdapConf{}, errors.New("LDAP Base DN 未配置")
	}

	uid := strings.TrimSpace(settings.UID)
	if uid == "" {
		uid = "uid"
	}
	scope := settings.Scope
	if scope == 0 {
		scope = 3
	}
	timeout := settings.ConnectionTimeout
	if timeout <= 0 {
		timeout = 30
	}

	return ergoldap.LdapConf{
		LdapURL:               strings.TrimSpace(settings.URL),
		LdapSearchDn:          strings.TrimSpace(settings.SearchDN),
		LdapSearchPassword:    settings.SearchPassword,
		LdapBaseDn:            strings.TrimSpace(settings.BaseDN),
		LdapFilter:            strings.TrimSpace(settings.Filter),
		LdapUID:               uid,
		LdapScope:             scope,
		LdapConnectionTimeout: timeout,
	}, nil
}

func ldapProfileFromSearchResult(result *goldap.SearchResult, uidAttr, fallbackUsername string) (ldapProfile, error) {
	if result == nil || len(result.Entries) != 1 {
		return ldapProfile{}, errors.New("LDAP 用户信息异常")
	}

	entry := result.Entries[0]
	profile := ldapProfile{Username: strings.TrimSpace(fallbackUsername)}
	uidAttr = strings.ToLower(strings.TrimSpace(uidAttr))
	for _, attr := range entry.Attributes {
		if len(attr.Values) == 0 {
			continue
		}
		name := strings.ToLower(attr.Name)
		value := strings.TrimSpace(attr.Values[0])
		if value == "" {
			continue
		}
		switch {
		case name == "mail" || name == "email":
			profile.Email = value
		case name == uidAttr:
			profile.Username = value
		case name == "displayname":
			profile.DisplayName = value
		case name == "department":
			profile.Department = value
		case name == "cn" && profile.DisplayName == "":
			profile.DisplayName = value
		case name == "uid" && profile.DisplayName == "":
			profile.DisplayName = value
		}
	}

	profile.Email = normalizeLDAPEmail(profile.Email)
	if profile.Email == "" || !strings.Contains(profile.Email, "@") {
		return ldapProfile{}, errors.New("LDAP 用户邮箱为空")
	}
	if len(profile.Email) > 50 {
		return ldapProfile{}, errors.New("LDAP 用户邮箱长度不能超过 50")
	}
	profile.DisplayName = trimRunes(strings.TrimSpace(profile.DisplayName), model.UserNameMaxLength)
	profile.Department = trimRunes(strings.TrimSpace(profile.Department), model.UserDepartmentMaxLength)
	profile.LDAPId = trimRunes(strings.TrimSpace(ldapBindingId(profile)), 256)
	return profile, nil
}

func ldapProfileFromUser(user ergoldap.LdapUser, fallbackUsername string) (ldapProfile, error) {
	profile := ldapProfile{
		Username:    strings.TrimSpace(user.Username),
		Email:       normalizeLDAPEmail(user.Email),
		DisplayName: strings.TrimSpace(user.DisplayName),
		Department:  strings.TrimSpace(user.Department),
	}
	if profile.Username == "" {
		profile.Username = strings.TrimSpace(fallbackUsername)
	}
	if profile.DisplayName == "" {
		profile.DisplayName = strings.TrimSpace(user.Realname)
	}
	if profile.Email == "" || !strings.Contains(profile.Email, "@") {
		return ldapProfile{}, errors.New("LDAP 用户邮箱为空")
	}
	if len(profile.Email) > 50 {
		return ldapProfile{}, errors.New("LDAP 用户邮箱长度不能超过 50")
	}
	profile.DisplayName = trimRunes(profile.DisplayName, model.UserNameMaxLength)
	profile.Department = trimRunes(profile.Department, model.UserDepartmentMaxLength)
	profile.LDAPId = trimRunes(strings.TrimSpace(ldapBindingId(profile)), 256)
	return profile, nil
}

func normalizeLDAPEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ldapBindingId(profile ldapProfile) string {
	return profile.Email
}

func findOrCreateLDAPUser(profile ldapProfile, allowCreate bool) (*model.User, error) {
	var user model.User
	err := model.DB.Where("email = ?", profile.Email).First(&user).Error
	if err == nil {
		return syncLDAPUserProfile(&user, profile)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if model.DB.Unscoped().Where("email = ?", profile.Email).Find(&model.User{}).RowsAffected > 0 {
		return nil, errors.New("LDAP 用户邮箱对应账号已删除")
	}
	if !allowCreate {
		return nil, errors.New("管理员关闭了新用户注册")
	}

	user = model.User{
		Username:    ldapLocalUsername(0, profile),
		DisplayName: ldapDisplayName(profile),
		Department:  profile.Department,
		LDAPId:      profile.LDAPId,
		Email:       profile.Email,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	if err := user.Insert(0); err != nil {
		return nil, err
	}
	return &user, nil
}

func syncLDAPUserProfile(user *model.User, profile ldapProfile) (*model.User, error) {
	updates := map[string]any{}
	if displayName := ldapDisplayName(profile); displayName != "" && user.DisplayName != displayName {
		updates["display_name"] = displayName
		user.DisplayName = displayName
	}
	if user.Department != profile.Department {
		updates["department"] = profile.Department
		user.Department = profile.Department
	}
	if profile.LDAPId != "" && user.LDAPId != profile.LDAPId {
		updates["ldap_id"] = profile.LDAPId
		user.LDAPId = profile.LDAPId
	}
	if len(updates) == 0 {
		return user, nil
	}
	if err := model.DB.Model(user).Updates(updates).Error; err != nil {
		return nil, err
	}
	_ = model.InvalidateUserCache(user.Id)
	return user, nil
}

func ldapLocalUsername(currentUserId int, profile ldapProfile) string {
	base := ldapDisplayName(profile)
	if base == "" {
		base = profile.Username
	}
	base = trimRunes(strings.TrimSpace(base), model.UserNameMaxLength)
	if base == "" {
		base = "ldap"
	}
	if isLDAPUsernameAvailable(currentUserId, base) {
		return base
	}

	baseRunes := []rune(base)
	for i := 1; i <= 999; i++ {
		suffix := fmt.Sprint(i)
		keep := model.UserNameMaxLength - len([]rune(suffix))
		if keep <= 0 {
			break
		}
		if len(baseRunes) < keep {
			keep = len(baseRunes)
		}
		candidate := string(baseRunes[:keep]) + suffix
		if isLDAPUsernameAvailable(currentUserId, candidate) {
			return candidate
		}
	}
	return "ldap_" + fmt.Sprint(model.GetMaxUserId()+1)
}

func isLDAPUsernameAvailable(currentUserId int, username string) bool {
	var user model.User
	err := model.DB.Unscoped().Select("id").Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	return err == nil && user.Id == currentUserId
}

func ldapDisplayName(profile ldapProfile) string {
	if profile.DisplayName != "" {
		return trimRunes(strings.TrimSpace(profile.DisplayName), model.UserNameMaxLength)
	}
	if profile.Username != "" {
		return trimRunes(strings.TrimSpace(profile.Username), model.UserNameMaxLength)
	}
	return "LDAP User"
}

func trimRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
