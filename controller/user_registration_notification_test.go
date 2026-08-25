package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
