package model

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRecordConsumeLogAlwaysRecordsClientMetadata(t *testing.T) {
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
	c.Request.Header["User-Agent"] = []string{"  codex-cli/1.2\x00  "}
	c.Set("username", "ip-user")

	other := map[string]interface{}{"request_path": "/v1/chat/completions", "user_agent": "forged"}
	RecordConsumeLog(c, 42, RecordConsumeLogParams{ModelName: "gpt-5", Quota: 10, Other: other})

	var log Log
	require.NoError(t, db.First(&log).Error)
	assert.Equal(t, "203.0.113.9", log.Ip)
	metadata, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, "codex-cli/1.2", metadata["user_agent"])
	assert.Equal(t, "/v1/chat/completions", metadata["request_path"])
	assert.Equal(t, "forged", other["user_agent"], "caller metadata must not be mutated")
}

func TestRecordErrorLogTruncatesRequestUserAgent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:log-error-ua-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	previousLogDB := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = previousLogDB })
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", strings.Repeat("x", 600))
	c.Set("username", "error-user")

	RecordErrorLog(c, 42, 7, "gpt-5", "token", "upstream failed", 3, 1, false, "default", nil)

	var log Log
	require.NoError(t, db.First(&log).Error)
	metadata, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	userAgent, ok := metadata["user_agent"].(string)
	require.True(t, ok)
	assert.Len(t, []rune(userAgent), 512)
}
