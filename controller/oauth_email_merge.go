package controller

import (
	"database/sql"
	"errors"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"gorm.io/gorm"
)

func mergeVerifiedOAuthEmail(provider oauth.Provider, oauthUser *oauth.OAuthUser) (*model.User, error) {
	var existing model.User
	found := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		owner, lookupErr := model.FindOAuthEmailOwnerWithTx(tx, oauthUser.Email)
		if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return nil
		}
		if lookupErr != nil {
			return lookupErr
		}
		existing = *owner
		found = true
		if existing.DeletedAt.Valid {
			return &OAuthUserDeletedError{}
		}
		if generic, ok := provider.(*oauth.GenericOAuthProvider); ok {
			var binding model.UserOAuthBinding
			getErr := tx.Where("user_id = ? AND provider_id = ?", existing.Id, generic.GetProviderId()).First(&binding).Error
			if getErr == nil {
				if binding.ProviderUserId != oauthUser.ProviderUserID {
					return errors.New("verified email is bound to another OAuth identity")
				}
				return nil
			}
			if !errors.Is(getErr, gorm.ErrRecordNotFound) {
				return getErr
			}
			return model.CreateUserOAuthBindingWithTx(tx, &model.UserOAuthBinding{
				UserId: existing.Id, ProviderId: generic.GetProviderId(), ProviderUserId: oauthUser.ProviderUserID,
			})
		}
		column := provider.ProviderUserIDColumn()
		if column == "" {
			return errors.New("OAuth provider does not expose a binding column")
		}
		if column == "dingtalk_id" {
			_, err := model.BindDingTalkIdentityWithTx(tx, existing.Id, oauthUser.ProviderUserID)
			return err
		}
		var current sql.NullString
		if err := tx.Model(&model.User{}).Where("id = ?", existing.Id).Select(column).Scan(&current).Error; err != nil {
			return err
		}
		if current.Valid && current.String != "" && current.String != oauthUser.ProviderUserID {
			return errors.New("verified email is bound to another OAuth identity")
		}
		return tx.Model(&model.User{}).Where("id = ?", existing.Id).Update(column, oauthUser.ProviderUserID).Error
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return model.GetUserById(existing.Id, false)
}
