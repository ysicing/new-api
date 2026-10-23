package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

var ErrDingTalkIdentityConflict = errors.New("DingTalk identity conflict")

func BindDingTalkIdentity(userId int, unionId string) (bool, error) {
	boundNow := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		boundNow, err = BindDingTalkIdentityWithTx(tx, userId, unionId)
		return err
	})
	return boundNow, err
}

// BindDingTalkIdentityWithTx 让注册、邮箱归并与主动绑定共享同一份身份所有权约束。
func BindDingTalkIdentityWithTx(tx *gorm.DB, userId int, unionId string) (bool, error) {
	unionId = strings.TrimSpace(unionId)
	if tx == nil || userId <= 0 || unionId == "" {
		return false, ErrDingTalkIdentityConflict
	}
	var user User
	if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
		return false, err
	}
	current := strings.TrimSpace(user.DingTalkId)
	if current != "" && current != unionId {
		return false, ErrDingTalkIdentityConflict
	}
	var owner User
	err := tx.Unscoped().Select("id").
		Where("dingtalk_id = ? AND id <> ?", unionId, userId).First(&owner).Error
	if err == nil {
		return false, ErrDingTalkIdentityConflict
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if err := ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderDingTalk, unionId, userId); err != nil {
		if errors.Is(err, ErrExternalIdentityAlreadyClaimed) {
			return false, ErrDingTalkIdentityConflict
		}
		return false, err
	}
	if current == unionId {
		return false, nil
	}
	err = tx.Model(&User{}).Where("id = ?", userId).Update("dingtalk_id", unionId).Error
	return err == nil, err
}
