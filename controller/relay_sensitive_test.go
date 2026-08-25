package controller

import (
	"bytes"
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
	var stdout bytes.Buffer
	previousWriter := gin.DefaultWriter
	common.LogWriterMu.Lock()
	gin.DefaultWriter = &stdout
	common.LogWriterMu.Unlock()
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
		common.LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		common.LogWriterMu.Unlock()
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

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	var response struct {
		Error types.OpenAIError `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, string(types.ErrorCodeSensitiveWordsDetected), response.Error.Code)
	assert.Contains(t, response.Error.Message, "请求触发敏感词审查，请修改后重试")

	var log model.Log
	require.NoError(t, db.First(&log).Error)
	assert.Equal(t, model.LogTypeError, log.Type)
	assert.Equal(t, 42, log.UserId)
	assert.Equal(t, "gpt-5", log.ModelName)
	assert.Equal(t, "req-sensitive", log.RequestId)
	assert.Contains(t, log.Content, "sensitive_words=blocked_word")
	assert.Contains(t, log.Content, "status_code=403")
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, string(types.ErrorCodeSensitiveWordsDetected), other["error_code"])
	assert.EqualValues(t, http.StatusForbidden, other["status_code"])
	assert.Equal(t, 42, notifiedUserId)
	assert.Equal(t, []string{"blocked_word"}, notifiedWords)
	assert.Contains(t, stdout.String(), `"event":"sensitive_word_prompt"`)
	assert.Contains(t, stdout.String(), `"prompt":"user\nblocked_word"`)
}

func TestNewSensitiveWordsDetectedErrorIsForbiddenAndNotRetryable(t *testing.T) {
	err := newSensitiveWordsDetectedError()

	assert.Equal(t, http.StatusForbidden, err.StatusCode)
	assert.Equal(t, types.ErrorCodeSensitiveWordsDetected, err.GetErrorCode())
	assert.True(t, types.IsSkipRetryError(err))
	assert.Equal(t, "请求触发敏感词审查，请修改后重试", err.Error())
}

func TestLogSensitivePromptAuditWritesCompleteSingleLineJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var stdout bytes.Buffer
	previousWriter := gin.DefaultWriter
	common.LogWriterMu.Lock()
	gin.DefaultWriter = &stdout
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		common.LogWriterMu.Unlock()
	})

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("id", 42)
	c.Set(common.RequestIdKey, "req-sensitive-prompt")
	prompt := "first line\n" + strings.Repeat("x", common.LocalLogContentLimit+100) + `"final line"`

	logSensitivePromptAudit(c, prompt)

	line := stdout.String()
	assert.Equal(t, 1, strings.Count(line, "\n"))
	var payload struct {
		Event     string `json:"event"`
		UserId    int    `json:"user_id"`
		RequestId string `json:"request_id"`
		Prompt    string `json:"prompt"`
	}
	require.NoError(t, common.Unmarshal([]byte(strings.TrimSpace(line)), &payload))
	assert.Equal(t, "sensitive_word_prompt", payload.Event)
	assert.Equal(t, 42, payload.UserId)
	assert.Equal(t, "req-sensitive-prompt", payload.RequestId)
	assert.Equal(t, prompt, payload.Prompt)
	assert.NotContains(t, line, "[truncated")
}
