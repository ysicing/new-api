package model

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRecordConsumeLogAlwaysRecordsClientIP(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:log-ip-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	previousLogDB, previousEnabled := LOG_DB, common.LogConsumeEnabled
	LOG_DB, common.LogConsumeEnabled = db, true
	t.Cleanup(func() { LOG_DB, common.LogConsumeEnabled = previousLogDB, previousEnabled })
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request.RemoteAddr = "203.0.113.9:1234"
	c.Set("username", "ip-user")

	RecordConsumeLog(c, 42, RecordConsumeLogParams{ModelName: "gpt-5", Quota: 10})

	var log Log
	require.NoError(t, db.First(&log).Error)
	assert.Equal(t, "203.0.113.9", log.Ip)
}
