package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ExternalIdentityProviderDingTalk = "dingtalk"
	ExternalIdentityProviderLDAP     = "ldap"
	ExternalIdentityProviderTelegram = "telegram"
)

var ErrExternalIdentityAlreadyClaimed = errors.New("external identity is already claimed")

// ExternalIdentityClaim is the durable ownership record for an identity issued
// by an external provider. The two unique indexes make both the provider
// subject and the user's provider slot single-owner without relying on a
// check-then-update sequence.
type ExternalIdentityClaim struct {
	Id        int64     `json:"id" gorm:"primaryKey"`
	Provider  string    `json:"provider" gorm:"type:varchar(32);not null;uniqueIndex:idx_external_identity_subject,priority:1;uniqueIndex:idx_external_identity_user,priority:1"`
	Subject   string    `json:"subject" gorm:"type:varchar(128);not null;uniqueIndex:idx_external_identity_subject,priority:2"`
	UserId    int       `json:"user_id" gorm:"not null;index;uniqueIndex:idx_external_identity_user,priority:2"`
	CreatedAt time.Time `json:"created_at"`
}

func (ExternalIdentityClaim) TableName() string {
	return "external_identity_claims"
}

// ClaimExternalIdentityWithTx atomically claims a provider subject for one
// user. Repeating the exact mapping is idempotent; every competing subject or
// user is rejected. Ownership is read back instead of trusting RowsAffected,
// whose duplicate-key semantics differ between supported databases.
func ClaimExternalIdentityWithTx(tx *gorm.DB, provider, subject string, userId int) error {
	provider = strings.TrimSpace(provider)
	subject = strings.TrimSpace(subject)
	if tx == nil || provider == "" || subject == "" || userId == 0 {
		return errors.New("external identity claim is invalid")
	}

	claim := ExternalIdentityClaim{Provider: provider, Subject: subject, UserId: userId}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
	if result.Error != nil {
		return result.Error
	}
	var subjectOwner ExternalIdentityClaim
	if err := tx.Where("provider = ? AND subject = ?", provider, subject).First(&subjectOwner).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrExternalIdentityAlreadyClaimed
		}
		return err
	}
	if subjectOwner.UserId != userId {
		return ErrExternalIdentityAlreadyClaimed
	}

	var userClaim ExternalIdentityClaim
	if err := tx.Where("provider = ? AND user_id = ?", provider, userId).First(&userClaim).Error; err != nil {
		return err
	}
	if userClaim.Subject != subject {
		return ErrExternalIdentityAlreadyClaimed
	}
	return nil
}

func ReleaseExternalIdentityWithTx(tx *gorm.DB, provider string, userId int) error {
	provider = strings.TrimSpace(provider)
	if tx == nil || provider == "" || userId == 0 {
		return errors.New("external identity release is invalid")
	}
	return tx.Where("provider = ? AND user_id = ?", provider, userId).
		Delete(&ExternalIdentityClaim{}).Error
}

func FindLDAPIdentityUsersWithTx(tx *gorm.DB, ldapID string) ([]User, error) {
	var users []User
	err := lockForUpdate(tx.Unscoped()).Where("ldap_id = ?", ldapID).Limit(2).Find(&users).Error
	return users, err
}

func FindExternalIdentityUserByEmailWithTx(tx *gorm.DB, email string) (*User, error) {
	email = NormalizeEmail(email)
	if email == "" {
		return nil, nil
	}
	var users []User
	if err := lockForUpdate(tx.Unscoped()).Where("LOWER(email) = ?", email).Limit(2).Find(&users).Error; err != nil {
		return nil, err
	}
	if len(users) > 1 {
		return nil, ErrEmailAlreadyTaken
	}
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

func ClaimLDAPIdentityWithTx(tx *gorm.DB, ldapID string, userID int) error {
	if strings.TrimSpace(ldapID) == "" {
		return ErrExternalIdentityAlreadyClaimed
	}
	subject := fmt.Sprintf("%x", sha256.Sum256([]byte(ldapID)))
	return ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderLDAP, subject, userID)
}

func GetUserByTelegramID(telegramID string) (*User, error) {
	var user User
	err := DB.Where("telegram_id = ?", telegramID).First(&user).Error
	return &user, err
}

// BindTelegramForSessionWithTx preserves single ownership and the session that
// started the binding. The caller consumes its OAuth flow in this transaction.
func BindTelegramForSessionWithTx(tx *gorm.DB, identity AuthSessionIdentity, telegramID string) error {
	if err := ValidateAuthSessionWithTx(tx, identity); err != nil {
		return err
	}
	var user User
	if err := tx.Select("id", "telegram_id").First(&user, identity.UserID).Error; err != nil {
		return err
	}
	if user.TelegramId != "" {
		return ErrExternalIdentityAlreadyClaimed
	}
	if err := ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderTelegram, telegramID, user.Id); err != nil {
		return err
	}
	result := tx.Model(&User{}).Where("id = ? AND (telegram_id = ? OR telegram_id IS NULL)", user.Id, "").Update("telegram_id", telegramID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrExternalIdentityAlreadyClaimed
	}
	return nil
}

func releaseAllExternalIdentitiesWithTx(tx *gorm.DB, userId int) error {
	if tx == nil || userId == 0 {
		return errors.New("external identity release is invalid")
	}
	return tx.Where("user_id = ?", userId).Delete(&ExternalIdentityClaim{}).Error
}

func InitializeExternalIdentityClaims() error {
	var users []User
	if err := DB.Unscoped().Select("id", "telegram_id", "dingtalk_id", "ldap_id").
		Where("telegram_id <> ? OR dingtalk_id <> ? OR ldap_id <> ?", "", "", "").Find(&users).Error; err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		for _, user := range users {
			for _, binding := range []struct{ provider, subject string }{
				{ExternalIdentityProviderTelegram, user.TelegramId},
				{ExternalIdentityProviderDingTalk, user.DingTalkId},
			} {
				if binding.subject == "" {
					continue
				}
				if err := ClaimExternalIdentityWithTx(tx, binding.provider, binding.subject, user.Id); err != nil {
					return fmt.Errorf("backfill %s identity for user %d: %w", binding.provider, user.Id, err)
				}
			}
			if user.LDAPId != "" {
				if err := ClaimLDAPIdentityWithTx(tx, user.LDAPId, user.Id); err != nil {
					return fmt.Errorf("backfill LDAP identity for user %d: %w", user.Id, err)
				}
			}
		}
		return nil
	})
}
