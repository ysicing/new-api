package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

func findOrCreateLDAPUser(profile ldapProfile, allowCreate bool) (*model.User, bool, error) {
	var user model.User
	created := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var matches []model.User
		if profile.LDAPId != "" {
			var err error
			matches, err = model.FindLDAPIdentityUsersWithTx(tx, profile.LDAPId)
			if err != nil {
				return err
			}
		}
		if len(matches) > 1 {
			return errors.New("ldap identity is ambiguous")
		}
		if len(matches) == 0 {
			matched, err := model.FindExternalIdentityUserByEmailWithTx(tx, profile.Email)
			if err != nil {
				return err
			}
			if matched != nil {
				matches = append(matches, *matched)
			}
		}
		if len(matches) == 0 {
			if !allowCreate {
				return errors.New("registration disabled")
			}
			user = model.User{
				Username: ldapLocalUsername(tx, profile), DisplayName: ldapDisplayName(profile),
				Department: profile.Department, LDAPId: profile.LDAPId, Email: profile.Email,
				Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
			}
			if err := user.InsertWithTx(tx, 0); err != nil {
				return err
			}
			created = true
		} else {
			user = matches[0]
			if user.DeletedAt.Valid || user.Status != common.UserStatusEnabled {
				return errors.New("ldap identity belongs to an inactive user")
			}
			if user.LDAPId != "" && user.LDAPId != profile.LDAPId {
				return errors.New("ldap email is bound to another identity")
			}
			updates := map[string]any{
				"display_name": ldapDisplayName(profile), "department": profile.Department, "ldap_id": profile.LDAPId,
			}
			if profile.Email != "" {
				var count int64
				if err := tx.Unscoped().Model(&model.User{}).Where("LOWER(email) = ? AND id <> ?", profile.Email, user.Id).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return model.ErrEmailAlreadyTaken
				}
				updates["email"] = profile.Email
			}
			if err := tx.Model(&model.User{}).Where("id = ?", user.Id).Updates(updates).Error; err != nil {
				return err
			}
			if err := tx.First(&user, user.Id).Error; err != nil {
				return err
			}
		}
		return model.ClaimLDAPIdentityWithTx(tx, profile.LDAPId, user.Id)
	})
	if err != nil {
		return nil, false, err
	}
	if created {
		user.FinalizeOAuthUserCreation(0)
	}
	return &user, created, nil
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

func ldapLocalUsername(tx *gorm.DB, profile ldapProfile) string {
	base := ldapDisplayName(profile)
	if ldapUsernameAvailable(tx, base) {
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
		if ldapUsernameAvailable(tx, candidate) {
			return candidate
		}
	}
	return "ldap_" + common.GetRandomString(12)
}

func ldapUsernameAvailable(tx *gorm.DB, username string) bool {
	var count int64
	return strings.TrimSpace(username) != "" && tx.Unscoped().Model(&model.User{}).Where("username = ?", username).Count(&count).Error == nil && count == 0
}
