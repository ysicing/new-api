package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

var ErrDingTalkIdentityConflict = errors.New("DingTalk identity conflict")

func BindDingTalkIdentity(userId int, unionId string) (bool, error) {
	unionId = strings.TrimSpace(unionId)
	if userId <= 0 || unionId == "" {
		return false, ErrDingTalkIdentityConflict
	}
	boundNow := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		current := strings.TrimSpace(user.DingTalkId)
		if current != "" && current != unionId {
			return ErrDingTalkIdentityConflict
		}
		var owner User
		err := tx.Unscoped().Select("id").
			Where("dingtalk_id = ? AND id <> ?", unionId, userId).First(&owner).Error
		if err == nil {
			return ErrDingTalkIdentityConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderDingTalk, unionId, userId); err != nil {
			if errors.Is(err, ErrExternalIdentityAlreadyClaimed) {
				return ErrDingTalkIdentityConflict
			}
			return err
		}
		if current == unionId {
			return nil
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("dingtalk_id", unionId).Error; err != nil {
			return err
		}
		boundNow = true
		return nil
	})
	return boundNow, err
}
