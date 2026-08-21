package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

func findOrCreateLDAPUser(profile ldapProfile, allowCreate bool) (*model.User, error) {
	var user model.User
	if profile.LDAPId != "" {
		err := model.DB.Unscoped().Where("ldap_id = ?", profile.LDAPId).First(&user).Error
		if err == nil {
			return syncLDAPProfile(&user, profile)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	if profile.Email != "" {
		err := model.DB.Unscoped().Where("LOWER(email) = ?", profile.Email).First(&user).Error
		if err == nil {
			return syncLDAPProfile(&user, profile)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	if !allowCreate {
		return nil, errors.New("registration disabled")
	}
	user = model.User{
		Username: ldapLocalUsername(profile), DisplayName: ldapDisplayName(profile),
		Department: profile.Department, LDAPId: profile.LDAPId, Email: profile.Email,
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	if err := user.Insert(0); err != nil {
		return nil, err
	}
	return &user, nil
}

func syncLDAPProfile(user *model.User, profile ldapProfile) (*model.User, error) {
	if user.DeletedAt.Valid {
		return nil, errors.New("ldap identity belongs to a deleted user")
	}
	updates := map[string]any{
		"display_name": ldapDisplayName(profile),
		"department":   profile.Department,
		"ldap_id":      profile.LDAPId,
	}
	if profile.Email != "" {
		if err := model.EnsureEmailAvailable(profile.Email, user.Id); err != nil {
			return nil, err
		}
		updates["email"] = profile.Email
	}
	if err := model.DB.Model(user).Updates(updates).Error; err != nil {
		return nil, err
	}
	return model.GetUserById(user.Id, false)
}

func ldapDisplayName(profile ldapProfile) string {
	if profile.DisplayName != "" {
		return trimLDAPRunes(profile.DisplayName, model.UserNameMaxLength)
	}
	if profile.Username != "" {
		return trimLDAPRunes(profile.Username, model.UserNameMaxLength)
	}
	return "LDAP User"
}

func ldapLocalUsername(profile ldapProfile) string {
	base := ldapDisplayName(profile)
	if ldapUsernameAvailable(base) {
		return base
	}
	baseRunes := []rune(base)
	for i := 1; i <= 999; i++ {
		suffix := fmt.Sprint(i)
		keep := model.UserNameMaxLength - len([]rune(suffix))
		if keep > len(baseRunes) {
			keep = len(baseRunes)
		}
		if keep <= 0 {
			break
		}
		candidate := string(baseRunes[:keep]) + suffix
		if ldapUsernameAvailable(candidate) {
			return candidate
		}
	}
	return "ldap_" + fmt.Sprint(model.GetMaxUserId()+1)
}

func ldapUsernameAvailable(username string) bool {
	var count int64
	return strings.TrimSpace(username) != "" && model.DB.Unscoped().Model(&model.User{}).Where("username = ?", username).Count(&count).Error == nil && count == 0
}
