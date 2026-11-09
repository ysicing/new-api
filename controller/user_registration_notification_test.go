package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLDAPLoginRequiresEncryptedPasswordWhenEnabled(t *testing.T) {
	setupManageUserTestDB(t)
	previousEncryption, previousAuthenticate := common.PasswordLoginEncryptionEnabled, authenticateLDAPUser
	common.PasswordLoginEncryptionEnabled = true
	authenticateLDAPUser = func(string, string) (*model.User, error) {
		t.Fatal("invalid encrypted credentials must not reach the LDAP directory")
		return nil, nil
	}
	t.Cleanup(func() {
		common.PasswordLoginEncryptionEnabled, authenticateLDAPUser = previousEncryption, previousAuthenticate
	})
	for _, body := range []string{
		`{"username":"alice","password":"plaintext"}`,
		`{"username":"alice","password_encrypted":"invalid","encryption_key_id":"unknown"}`,
	} {
		response := securityEnrollmentRequest("POST", "/api/user/ldap/login", body, "", service.AuthIdentity{}, LDAPLogin)
		assert.Contains(t, response.Body.String(), `"success":false`)
		assert.Empty(t, response.Header().Values("Set-Cookie"))
	}
}

func TestLDAPLoginPreservesUnifiedMFAVerification(t *testing.T) {
	for _, factor := range []string{"2fa", "passkey"} {
		t.Run(factor, func(t *testing.T) {
			user, _ := setupSecurityEnrollmentTest(t)
			if factor == "2fa" {
				require.NoError(t, model.DB.Create(&model.TwoFA{UserId: user.Id, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true}).Error)
			} else {
				newSecurityLoginPasskey(t, user.Id)
			}
			previousAuthenticate := authenticateLDAPUser
			authenticateLDAPUser = func(username, password string) (*model.User, error) {
				assert.Equal(t, "alice", username)
				assert.Equal(t, "directory-secret", password)
				return user, nil
			}
			t.Cleanup(func() { authenticateLDAPUser = previousAuthenticate })
			response := securityEnrollmentRequest("POST", "/api/user/ldap/login", `{"username":"alice","password":"directory-secret"}`, "", service.AuthIdentity{}, LDAPLogin)
			var result struct {
				Success bool                   `json:"success"`
				Data    service.LoginChallenge `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			require.True(t, result.Success, response.Body.String())
			assert.True(t, result.Data.RequireVerification)
			assert.NotEmpty(t, result.Data.FlowToken)
			assert.Empty(t, response.Header().Values("Set-Cookie"))
			var sessions int64
			require.NoError(t, model.DB.Model(&model.UserSession{}).Where("user_id = ?", user.Id).Count(&sessions).Error)
			assert.EqualValues(t, 1, sessions)
		})
	}
}

func TestPasswordRegistrationNotifiesNewUser(t *testing.T) {
	setupManageUserTestDB(t)
	previousRegisterEnabled := common.RegisterEnabled
	previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
	previousEmailVerificationEnabled := common.EmailVerificationEnabled
	previousQuotaPoolEnabled := common.QuotaPoolEnabled
	previousGenerateDefaultToken := constant.GenerateDefaultToken
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaPoolEnabled = false
	constant.GenerateDefaultToken = false
	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
		common.EmailVerificationEnabled = previousEmailVerificationEnabled
		common.QuotaPoolEnabled = previousQuotaPoolEnabled
		constant.GenerateDefaultToken = previousGenerateDefaultToken
	})

	previousNotify := notifyNewSelfRegisteredUser
	notifiedUserId := 0
	notifyNewSelfRegisteredUser = func(userId int) { notifiedUserId = userId }
	t.Cleanup(func() { notifyNewSelfRegisteredUser = previousNotify })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(`{"username":"new-member","password":"password"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	Register(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	require.Positive(t, notifiedUserId)
}

func TestWeChatRegistrationNotifiesNewUser(t *testing.T) {
	setupManageUserTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"success":true,"data":"wechat-new-user"}`)
	}))
	t.Cleanup(server.Close)

	previousRegisterEnabled := common.RegisterEnabled
	previousWeChatAuthEnabled := common.WeChatAuthEnabled
	previousQuotaPoolEnabled := common.QuotaPoolEnabled
	previousServerAddress := common.WeChatServerAddress
	common.RegisterEnabled = true
	common.WeChatAuthEnabled = true
	common.QuotaPoolEnabled = false
	common.WeChatServerAddress = server.URL
	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.WeChatAuthEnabled = previousWeChatAuthEnabled
		common.QuotaPoolEnabled = previousQuotaPoolEnabled
		common.WeChatServerAddress = previousServerAddress
	})

	previousNotify := notifyNewSelfRegisteredUser
	notifiedUserId := 0
	notifyNewSelfRegisteredUser = func(userId int) { notifiedUserId = userId }
	t.Cleanup(func() { notifyNewSelfRegisteredUser = previousNotify })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/wechat?code=ok", nil)
	WeChatAuth(c)

	require.Positive(t, notifiedUserId)
}
