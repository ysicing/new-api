package controller

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"gorm.io/gorm"
)

func mergeVerifiedOAuthEmail(provider oauth.Provider, oauthUser *oauth.OAuthUser) (*model.User, error) {
	if !oauthUser.EmailVerified || model.NormalizeEmail(oauthUser.Email) == "" {
		return nil, nil
	}
	var user *model.User
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		user, err = model.FindExternalIdentityUserByEmailWithTx(tx, oauthUser.Email)
		if err != nil || user == nil {
			return err
		}
		if user.DeletedAt.Valid {
			return &OAuthUserDeletedError{}
		}
		if user.Status != common.UserStatusEnabled {
			return model.ErrUserSessionInactive
		}
		if oauthUser.ProviderUserID == "" {
			return model.ErrAccountBindingChanged
		}
		if genericProvider, ok := provider.(*oauth.GenericOAuthProvider); ok {
			var binding model.UserOAuthBinding
			err := tx.Where("user_id = ? AND provider_id = ?", user.Id, genericProvider.GetProviderId()).First(&binding).Error
			if err == nil {
				if binding.ProviderUserId != oauthUser.ProviderUserID {
					return model.ErrAccountBindingChanged
				}
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return model.CreateUserOAuthBindingWithTx(tx, &model.UserOAuthBinding{
				UserId: user.Id, ProviderId: genericProvider.GetProviderId(), ProviderUserId: oauthUser.ProviderUserID,
			})
		}
		column := provider.ProviderUserIDColumn()
		currentID, supported := map[string]string{
			"github_id": user.GitHubId, "discord_id": user.DiscordId, "oidc_id": user.OidcId,
			"dingtalk_id": user.DingTalkId, "linux_do_id": user.LinuxDOId, "wechat_id": user.WeChatId,
		}[column]
		if !supported || (currentID != "" && currentID != oauthUser.ProviderUserID) {
			return model.ErrAccountBindingChanged
		}
		if column == "dingtalk_id" {
			if _, err := model.BindDingTalkIdentityWithTx(tx, user.Id, oauthUser.ProviderUserID); err != nil {
				return err
			}
		} else {
			var owners int64
			if err := tx.Unscoped().Model(&model.User{}).Where(column+" = ? AND id <> ?", oauthUser.ProviderUserID, user.Id).Count(&owners).Error; err != nil {
				return err
			}
			if owners > 0 {
				return model.ErrAccountBindingChanged
			}
			if err := tx.Model(&model.User{}).Where("id = ?", user.Id).Update(column, oauthUser.ProviderUserID).Error; err != nil {
				return err
			}
		}
		provider.SetProviderUserID(user, oauthUser.ProviderUserID)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func checkOAuthRegistrationEmail(email string) error {
	if model.NormalizeEmail(email) == "" {
		return nil
	}
	err := model.EnsureEmailAvailable(email, 0)
	if errors.Is(err, model.ErrEmailAlreadyTaken) {
		return &OAuthEmailAlreadyTakenError{}
	}
	return err
}
