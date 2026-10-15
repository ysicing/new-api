package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateMagpieMonitorTaskEnqueuesManualCheckAndRejectsActiveTask(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}, &model.Log{}))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/system-task/magpie-monitor", nil)
	CreateMagpieMonitorSystemTask(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	task, err := model.GetActiveSystemTask(model.SystemTaskTypeMagpieChannelMonitor)
	require.NoError(t, err)
	require.NotNil(t, task)
	var payload service.MagpieChannelMonitorPayload
	require.NoError(t, task.DecodePayload(&payload))
	assert.True(t, payload.Manual)

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/system-task/magpie-monitor", nil)
	CreateMagpieMonitorSystemTask(ctx)
	require.Equal(t, http.StatusConflict, recorder.Code)
	var response struct {
		Success bool                     `json:"success"`
		Data    model.SystemTaskResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	assert.Equal(t, task.TaskID, response.Data.TaskID)
	var count int64
	require.NoError(t, db.Model(&model.SystemTask{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}
