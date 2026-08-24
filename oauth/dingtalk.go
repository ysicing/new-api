package oauth

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

const dingTalkAccessTokenHeader = "x-acs-dingtalk-access-token"

var (
	dingTalkTokenEndpoint    = "https://api.dingtalk.com/v1.0/oauth2/userAccessToken"
	dingTalkUserInfoEndpoint = "https://api.dingtalk.com/v1.0/contact/users/me"
)

func init() {
	Register("dingtalk", &DingTalkProvider{})
}

// DingTalkProvider 实现钉钉企业内部应用的 OAuth2 扫码登录协议。
type DingTalkProvider struct{}

type dingTalkTokenRequest struct {
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Code         string `json:"code"`
	GrantType    string `json:"grantType"`
}

type dingTalkTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpireIn     int    `json:"expireIn"`
	CorpId       string `json:"corpId"`
}

type dingTalkUser struct {
	UnionId string `json:"unionId"`
	OpenId  string `json:"openId"`
	Nick    string `json:"nick"`
	Email   string `json:"email"`
}

func (p *DingTalkProvider) GetName() string { return "DingTalk" }

func (p *DingTalkProvider) IsEnabled() bool {
	return system_setting.GetDingTalkSettings().Enabled
}

func (p *DingTalkProvider) ExchangeToken(ctx context.Context, code string, _ *gin.Context) (*OAuthToken, error) {
	if strings.TrimSpace(code) == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	settings := system_setting.GetDingTalkSettings()
	payload, err := common.Marshal(dingTalkTokenRequest{
		ClientId: settings.ClientId, ClientSecret: settings.ClientSecret,
		Code: code, GrantType: "authorization_code",
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dingTalkTokenEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": p.GetName()}, err.Error())
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": p.GetName()}, fmt.Sprintf("DingTalk token status %d", res.StatusCode))
	}

	var response dingTalkTokenResponse
	if err := common.DecodeJson(res.Body, &response); err != nil {
		return nil, err
	}
	if response.AccessToken == "" {
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": p.GetName()})
	}
	if strings.TrimSpace(response.CorpId) != strings.TrimSpace(settings.CorpId) {
		logger.LogWarn(ctx, "[OAuth-DingTalk] rejected user from another enterprise")
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthGetUserErr, map[string]any{"Provider": p.GetName()}, "DingTalk enterprise mismatch")
	}

	return &OAuthToken{
		AccessToken: response.AccessToken, RefreshToken: response.RefreshToken,
		ExpiresIn: response.ExpireIn, TokenType: "Bearer",
	}, nil
}

func (p *DingTalkProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	if token == nil || token.AccessToken == "" {
		return nil, NewOAuthError(i18n.MsgOAuthGetUserErr, map[string]any{"Provider": p.GetName()})
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dingTalkUserInfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(dingTalkAccessTokenHeader, token.AccessToken)
	req.Header.Set("Accept", "application/json")

	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": p.GetName()}, err.Error())
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthGetUserErr, map[string]any{"Provider": p.GetName()}, fmt.Sprintf("DingTalk user info status %d", res.StatusCode))
	}

	var user dingTalkUser
	if err := common.DecodeJson(res.Body, &user); err != nil {
		return nil, err
	}
	if strings.TrimSpace(user.UnionId) == "" {
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": p.GetName()})
	}
	email := model.NormalizeEmail(user.Email)
	return &OAuthUser{
		ProviderUserID:              user.UnionId,
		Username:                    strings.Split(email, "@")[0],
		DisplayName:                 strings.TrimSpace(user.Nick),
		Email:                       email,
		EmailVerified:               email != "",
		RequireEmailForRegistration: true,
	}, nil
}

func (p *DingTalkProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsDingTalkIdAlreadyTaken(providerUserID)
}

func (p *DingTalkProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.DingTalkId = providerUserID
	return user.FillUserByDingTalkId()
}

func (p *DingTalkProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.DingTalkId = providerUserID
}

func (p *DingTalkProvider) GetProviderPrefix() string { return "dingtalk_" }

func (p *DingTalkProvider) ProviderUserIDColumn() string { return "dingtalk_id" }
