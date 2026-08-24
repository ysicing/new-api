package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListDingTalkTestUsersReturnsSearchPage(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	users := []model.User{
		{Username: "alice", DisplayName: "Alice", Email: "alice@example.com", Department: "研发部", Password: "password", AffCode: "bot-user-alice", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
		{Username: "bob", DisplayName: "Bob", Email: "bob@example.com", Department: "平台部", Password: "password", AffCode: "bot-user-bob", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}
	require.NoError(t, db.Create(&users).Error)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/dingtalk/test-users?keyword=研发&p=1&page_size=20", nil)

	ListDingTalkTestUsers(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"username":"alice"`)
	assert.NotContains(t, recorder.Body.String(), `"username":"bob"`)
}

func TestSendDingTalkTestMessageReturnsBindingStatus(t *testing.T) {
	previous := sendDingTalkTestMessage
	sendDingTalkTestMessage = func(_ context.Context, userId int) (service.DingTalkTestMessageResult, error) {
		assert.Equal(t, 12, userId)
		return service.DingTalkTestMessageResult{BoundNow: true}, nil
	}
	t.Cleanup(func() { sendDingTalkTestMessage = previous })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/dingtalk/test-message", strings.NewReader(`{"user_id":12}`))

	SendDingTalkTestMessage(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"bound_now":true`)
}

func TestSendDingTalkTestMessageReturnsStableConfigurationError(t *testing.T) {
	previous := sendDingTalkTestMessage
	sendDingTalkTestMessage = func(context.Context, int) (service.DingTalkTestMessageResult, error) {
		return service.DingTalkTestMessageResult{}, service.ErrDingTalkNotConfigured
	}
	t.Cleanup(func() { sendDingTalkTestMessage = previous })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/dingtalk/test-message", strings.NewReader(`{"user_id":12}`))

	SendDingTalkTestMessage(c)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"DINGTALK_NOT_CONFIGURED"`)
}

func TestSendDingTalkTestMessageRejectsInvalidUserId(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/dingtalk/test-message", strings.NewReader(`{"user_id":0}`))

	SendDingTalkTestMessage(c)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"DINGTALK_INVALID_USER"`)
}
