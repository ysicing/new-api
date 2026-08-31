package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type modelStatisticsResponse struct {
	Success bool                   `json:"success"`
	Data    []model.ModelUsageStat `json:"data"`
}

func setupModelStatisticsControllerTest(t *testing.T) time.Time {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaData{}))
	now := time.Now()
	require.NoError(t, db.Create(&[]model.QuotaData{
		{UserID: 1, ModelName: "gpt-5", CreatedAt: now.Unix(), Count: 2, Quota: 40},
		{UserID: 2, ModelName: "claude-4", CreatedAt: now.Unix(), Count: 3, Quota: 60},
	}).Error)
	return now
}

func callModelStatistics(t *testing.T, role, userId int, scope string) modelStatisticsResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("role", role)
	c.Set("id", userId)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/data/model-statistics?period=week&scope="+scope, nil)
	GetModelStatistics(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response modelStatisticsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response
}

func TestGetModelStatisticsForcesOrdinaryUserToSelfScope(t *testing.T) {
	setupModelStatisticsControllerTest(t)

	response := callModelStatistics(t, common.RoleCommonUser, 1, "all")

	require.Len(t, response.Data, 1)
	assert.Equal(t, "gpt-5", response.Data[0].ModelName)
}

func TestGetModelStatisticsAllowsAdminAllAndSelfScopes(t *testing.T) {
	setupModelStatisticsControllerTest(t)

	all := callModelStatistics(t, common.RoleAdminUser, 1, "all")
	self := callModelStatistics(t, common.RoleAdminUser, 1, "self")

	require.Len(t, all.Data, 2)
	require.Len(t, self.Data, 1)
	assert.Equal(t, "gpt-5", self.Data[0].ModelName)
}
