package oauth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDingTalkProviderTest(t *testing.T, handler http.Handler) *DingTalkProvider {
	t.Helper()

	server := httptest.NewServer(handler)
	previousTokenEndpoint := dingTalkTokenEndpoint
	previousUserInfoEndpoint := dingTalkUserInfoEndpoint
	previousSettings := *system_setting.GetDingTalkSettings()
	dingTalkTokenEndpoint = server.URL + "/token"
	dingTalkUserInfoEndpoint = server.URL + "/me"
	*system_setting.GetDingTalkSettings() = system_setting.DingTalkSettings{
		Enabled: true, CorpId: "corp-1", ClientId: "app-key", ClientSecret: "app-secret",
	}
	t.Cleanup(func() {
		server.Close()
		dingTalkTokenEndpoint = previousTokenEndpoint
		dingTalkUserInfoEndpoint = previousUserInfoEndpoint
		*system_setting.GetDingTalkSettings() = previousSettings
	})

	return &DingTalkProvider{}
}

func TestDingTalkProviderExchangesTokenAndLoadsEmployee(t *testing.T) {
	provider := setupDingTalkProviderTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			var payload map[string]string
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			assert.Equal(t, map[string]string{
				"clientId": "app-key", "clientSecret": "app-secret", "code": "auth-code", "grantType": "authorization_code",
			}, payload)
			_, _ = fmt.Fprint(w, `{"accessToken":"user-token","refreshToken":"refresh-token","expireIn":7200,"corpId":"corp-1"}`)
		case "/me":
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "user-token", r.Header.Get("x-acs-dingtalk-access-token"))
			_, _ = fmt.Fprint(w, `{"unionId":"union-1","openId":"open-1","nick":"Alice","email":"Alice@Example.com"}`)
		default:
			http.NotFound(w, r)
		}
	}))

	token, err := provider.ExchangeToken(context.Background(), "auth-code", nil)
	require.NoError(t, err)
	assert.Equal(t, "user-token", token.AccessToken)

	user, err := provider.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "union-1", user.ProviderUserID)
	assert.Equal(t, "Alice", user.DisplayName)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.True(t, user.EmailVerified)
	assert.True(t, user.RequireEmailForRegistration)
}

func TestDingTalkProviderRejectsAnotherEnterprise(t *testing.T) {
	provider := setupDingTalkProviderTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"accessToken":"user-token","expireIn":7200,"corpId":"corp-other"}`)
	}))

	_, err := provider.ExchangeToken(context.Background(), "auth-code", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enterprise")
}
