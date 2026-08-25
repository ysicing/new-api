package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuotaPoolMemberAddEndpointsNotifyJoinedUser(t *testing.T) {
	tests := []struct {
		name string
		call func(*gin.Context)
	}{
		{name: "global administration", call: AddQuotaPoolMember},
		{name: "pool administration", call: AddSelfQuotaPoolMember},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupManageUserTestDB(t)
			setQuotaPoolFeatureForTest(t)
			source := model.QuotaPool{
				Name: model.QuotaPoolNewUserName, PoolType: model.QuotaPoolTypeNewUser,
				Enabled: true, BaseQuota: -1, Quota: -1,
			}
			target := model.QuotaPool{
				Name: "研发额度池", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
				AutoRechargeAmount: model.QuotaPoolAutoRechargeOff,
			}
			require.NoError(t, db.Create(&source).Error)
			require.NoError(t, db.Create(&target).Error)
			operator := model.User{
				Username: "pool-operator", Password: "password", AffCode: "pool-operator-aff",
				Role: common.RoleRootUser, Status: common.UserStatusEnabled, QuotaPoolId: target.Id,
			}
			member := model.User{
				Username: "pool-candidate", Password: "password", AffCode: "pool-candidate-aff",
				Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: source.Id,
			}
			require.NoError(t, db.Create(&operator).Error)
			require.NoError(t, db.Create(&member).Error)
			if test.name == "pool administration" {
				require.NoError(t, db.Create(&model.QuotaPoolAdmin{
					PoolId: target.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevel,
				}).Error)
			}

			previousNotify := notifyQuotaPoolMemberJoined
			notifiedUserId, notifiedPoolId := 0, 0
			notifyQuotaPoolMemberJoined = func(userId, poolId int) {
				notifiedUserId, notifiedPoolId = userId, poolId
			}
			t.Cleanup(func() { notifyQuotaPoolMemberJoined = previousNotify })

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/quota-pools/members", strings.NewReader(fmt.Sprintf(`{"user_id":%d}`, member.Id)))
			c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", target.Id)}}
			c.Set("id", operator.Id)
			c.Set("username", operator.Username)
			c.Set("role", operator.Role)

			test.call(c)

			assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, member.Id, notifiedUserId)
			assert.Equal(t, target.Id, notifiedPoolId)
		})
	}
}
