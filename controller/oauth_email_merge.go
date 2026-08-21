package controller

import (
	"errors"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"gorm.io/gorm"
)

func mergeVerifiedOAuthEmail(provider oauth.Provider, oauthUser *oauth.OAuthUser) (*model.User, error) {
	var existing model.User
	err := model.DB.Unscoped().Where("LOWER(email) = ?", oauthUser.Email).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if existing.DeletedAt.Valid {
		return nil, &OAuthUserDeletedError{}
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if generic, ok := provider.(*oauth.GenericOAuthProvider); ok {
			binding, getErr := model.GetUserOAuthBinding(existing.Id, generic.GetProviderId())
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
		var current string
		if err := tx.Model(&model.User{}).Where("id = ?", existing.Id).Select(column).Scan(&current).Error; err != nil {
			return err
		}
		if current != "" && current != oauthUser.ProviderUserID {
			return errors.New("verified email is bound to another OAuth identity")
		}
		return tx.Model(&model.User{}).Where("id = ?", existing.Id).Update(column, oauthUser.ProviderUserID).Error
	})
	if err != nil {
		return nil, err
	}
	return model.GetUserById(existing.Id, false)
}
