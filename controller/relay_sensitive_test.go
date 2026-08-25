package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRelayRecordsSensitiveWordRejectionAsErrorLog(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:relay-sensitive-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}))

	previousLogDB := model.LOG_DB
	previousErrorLogEnabled := constant.ErrorLogEnabled
	previousCheckSensitiveEnabled := setting.CheckSensitiveEnabled
	previousCheckSensitiveOnPromptEnabled := setting.CheckSensitiveOnPromptEnabled
	previousSensitiveWords := setting.SensitiveWords
	model.LOG_DB = db
	constant.ErrorLogEnabled = true
	setting.CheckSensitiveEnabled = true
	setting.CheckSensitiveOnPromptEnabled = true
	setting.SensitiveWords = []string{"blocked_word"}
	previousNotify := notifySensitiveWordsDetected
	notifiedUserId := 0
	var notifiedWords []string
	notifySensitiveWordsDetected = func(userId int, words []string) {
		notifiedUserId = userId
		notifiedWords = append([]string(nil), words...)
	}
	t.Cleanup(func() {
		model.LOG_DB = previousLogDB
		constant.ErrorLogEnabled = previousErrorLogEnabled
		setting.CheckSensitiveEnabled = previousCheckSensitiveEnabled
		setting.CheckSensitiveOnPromptEnabled = previousCheckSensitiveOnPromptEnabled
		setting.SensitiveWords = previousSensitiveWords
		notifySensitiveWordsDetected = previousNotify
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5","messages":[{"role":"user","content":"blocked_word"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 42)
	c.Set("username", "sensitive-user")
	c.Set("token_id", 7)
	c.Set("token_name", "sensitive-token")
	c.Set("original_model", "gpt-5")
	c.Set("group", "default")
	c.Set(string(constant.ContextKeyRequestStartTime), time.Now())
	c.Set(common.RequestIdKey, "req-sensitive")

	Relay(c, types.RelayFormatOpenAI)

	var log model.Log
	require.NoError(t, db.First(&log).Error)
	assert.Equal(t, model.LogTypeError, log.Type)
	assert.Equal(t, 42, log.UserId)
	assert.Equal(t, "gpt-5", log.ModelName)
	assert.Equal(t, "req-sensitive", log.RequestId)
	assert.Contains(t, log.Content, "sensitive_words=blocked_word")
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, string(types.ErrorCodeSensitiveWordsDetected), other["error_code"])
	assert.Equal(t, 42, notifiedUserId)
	assert.Equal(t, []string{"blocked_word"}, notifiedWords)
}
