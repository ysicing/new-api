package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootMovesOwnQuotaPoolAndRecordsAudit(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.QuotaPool{}, &model.QuotaPoolAdmin{}, &model.QuotaPoolTransaction{}))
	source := model.QuotaPool{Name: "root-disabled-source", PoolType: model.QuotaPoolTypeNormal, Enabled: false, Quota: 100}
	target := model.QuotaPool{Name: "root-enabled-target", PoolType: model.QuotaPoolTypeNormal, Enabled: true, Quota: 200}
	require.NoError(t, db.Create(&source).Error)
	require.NoError(t, db.Model(&source).Update("enabled", false).Error)
	require.NoError(t, db.Create(&target).Error)
	root := model.User{Username: "root-move", AffCode: "root-move", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Quota: 25, QuotaPoolId: source.Id}
	require.NoError(t, db.Create(&root).Error)
	setQuotaPoolFeatureForTest(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/quota_pool/users/"+strconv.Itoa(root.Id),
		strings.NewReader(fmt.Sprintf(`{"pool_id":%d}`, target.Id)))
	c.Params = gin.Params{{Key: "user_id", Value: strconv.Itoa(root.Id)}}
	c.Set("id", root.Id)
	c.Set("username", root.Username)
	c.Set("role", common.RoleRootUser)

	MoveUserQuotaPool(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	require.NoError(t, db.First(&root, root.Id).Error)
	assert.Equal(t, common.RoleRootUser, root.Role)
	assert.Equal(t, target.Id, root.QuotaPoolId)
	assert.Zero(t, root.Quota)
	require.NoError(t, db.First(&source, source.Id).Error)
	assert.Equal(t, 125, source.Quota)
	other := lastQuotaPoolAudit(t, db)
	assert.Equal(t, "quota_pool.member_move", other.Op.Action)
	assert.EqualValues(t, root.Id, other.Op.Params["user_id"])
	assert.EqualValues(t, target.Id, other.Op.Params["quota_pool_id"])
	var transaction model.QuotaPoolTransaction
	require.NoError(t, db.First(&transaction).Error)
	assert.Equal(t, model.QuotaPoolTransactionReclaimUser, transaction.Type)
	assert.Equal(t, 25, transaction.Amount)
}
