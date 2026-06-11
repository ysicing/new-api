package model

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

type Log struct {
	Id               int    `json:"id" gorm:"index:idx_created_at_id,priority:1;index:idx_user_id_id,priority:2"`
	UserId           int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1;index:idx_logs_type_created_at_user_id,priority:3;index:idx_logs_user_id_type_created_at_id,priority:1;index:idx_logs_user_id_created_at_id,priority:1"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:2;index:idx_created_at_type;index:idx_logs_type_created_at_user_id,priority:2;index:idx_logs_user_id_type_created_at_id,priority:3;index:idx_logs_username_type_created_at_id,priority:3;index:idx_logs_user_id_created_at_id,priority:2;index:idx_logs_username_created_at_id,priority:2"`
	Type             int    `json:"type" gorm:"index:idx_created_at_type;index:idx_logs_type_created_at_user_id,priority:1;index:idx_logs_user_id_type_created_at_id,priority:2;index:idx_logs_username_type_created_at_id,priority:2"`
	Content          string `json:"content"`
	Username         string `json:"username" gorm:"index;index:index_username_model_name,priority:2;index:idx_logs_username_type_created_at_id,priority:1;index:idx_logs_username_created_at_id,priority:1;default:''"`
	TokenName        string `json:"token_name" gorm:"index;default:''"`
	ModelName        string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota            int    `json:"quota" gorm:"default:0"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`
	UseTime          int    `json:"use_time" gorm:"default:0"`
	IsStream         bool   `json:"is_stream"`
	ChannelId        int    `json:"channel" gorm:"index"`
	ChannelName      string `json:"channel_name" gorm:"->"`
	TokenId          int    `json:"token_id" gorm:"default:0;index"`
	Group            string `json:"group" gorm:"index"`
	Ip               string `json:"ip" gorm:"index;default:''"`
	RequestId        string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	Other            string `json:"other"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
)

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			delete(otherMap, "reject_reason")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = startIdx + i + 1
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	err = LOG_DB.Model(&Log{}).Where("token_id = ?", tokenId).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func CountAutoRechargeLogs(userId int, sinceTimestamp int64) (int64, error) {
	var count int64
	err := LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ? AND content LIKE ? AND created_at >= ?",
			userId, LogTypeSystem, "系统自动赠送%", sinceTimestamp).
		Count(&count).Error
	return count, err
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, content))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	otherStr := common.MapToJsonStr(other)
	// 判断是否需要记录 IP
	needRecordIp := true
	// if settingMap, err := GetUserSetting(userId, false); err == nil {
	// 	if settingMap.RecordIpLog {
	// 		needRecordIp = true
	// 	}
	// }
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId: requestId,
		Other:     otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	otherStr := common.MapToJsonStr(params.Other)
	// 判断是否需要记录 IP
	needRecordIp := true
	// if settingMap, err := GetUserSetting(userId, false); err == nil {
	// 	if settingMap.RecordIpLog {
	// 		needRecordIp = true
	// 	}
	// }
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId: requestId,
		Other:     otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
}

type RecordTaskBillingLogParams struct {
	UserId    int
	LogType   int
	Content   string
	ChannelId int
	ModelName string
	Quota     int
	TokenId   int
	Group     string
	Other     map[string]interface{}
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	log := &Log{
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Other:     common.MapToJsonStr(params.Other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
	}
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("logs.type = ?", logType)
	}

	if modelName != "" {
		tx = tx.Where("logs.model_name like ?", modelName)
	}
	if username != "" {
		tx = tx.Where("logs.username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return nil, 0, err
		}
		tx = tx.Where("logs.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, startIdx)
	return logs, total, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

type UserQuotaStat struct {
	UserId         int    `json:"user_id" gorm:"column:user_id"`
	Username       string `json:"username" gorm:"column:username"`
	RemainingQuota int    `json:"remaining_quota" gorm:"column:remaining_quota"`
	TotalQuota     int    `json:"total_quota" gorm:"column:total_quota"`
	UsedQuota      int    `json:"used_quota" gorm:"column:used_quota"`
	GptQuota       int    `json:"gpt_quota" gorm:"column:gpt_quota"`
	ClaudeQuota    int    `json:"claude_quota" gorm:"column:claude_quota"`
	DeepSeekQuota  int    `json:"deepseek_quota" gorm:"column:deepseek_quota"`
	GeminiQuota    int    `json:"gemini_quota" gorm:"column:gemini_quota"`
	QwenQuota      int    `json:"qwen_quota" gorm:"column:qwen_quota"`
	OtherQuota     int    `json:"other_quota" gorm:"column:other_quota"`
}

func GetTopUsers(startTimestamp int64, endTimestamp int64, modelName string, limit int) ([]UserQuotaStat, error) {
	if limit <= 0 {
		limit = 10
	} else if limit > 30 {
		limit = 30
	}
	if startTimestamp > 0 && endTimestamp > 0 {
		maxDuration := int64(30 * 24 * 60 * 60)
		if endTimestamp-startTimestamp > maxDuration {
			startTimestamp = endTimestamp - maxDuration
		}
	}

	currentHourStart := currentHourStartTimestamp()

	if DB == LOG_DB {
		results, err := getTopUsersFromUsageUnion(startTimestamp, endTimestamp, modelName, currentHourStart, limit)
		if err != nil {
			return nil, err
		}
		if len(results) > 0 {
			return results, nil
		}
		return getTopUsersFromLogs(startTimestamp, endTimestamp, modelName, limit)
	}

	return getTopUsersMergedAcrossDB(startTimestamp, endTimestamp, modelName, currentHourStart, limit)
}

func topUsersSettledRange(startTimestamp int64, endTimestamp int64, currentHourStart int64) (int64, int64, bool) {
	if currentHourStart <= 0 {
		return 0, 0, false
	}
	settledEndTimestamp := currentHourStart - 1
	if endTimestamp != 0 && endTimestamp < currentHourStart {
		settledEndTimestamp = endTimestamp
	}
	if startTimestamp != 0 && startTimestamp > settledEndTimestamp {
		return 0, 0, false
	}
	return startTimestamp, settledEndTimestamp, true
}

func topUsersCurrentHourRange(startTimestamp int64, endTimestamp int64, currentHourStart int64) (int64, int64, bool) {
	if endTimestamp != 0 && endTimestamp < currentHourStart {
		return 0, 0, false
	}
	if startTimestamp > currentHourStart {
		return startTimestamp, endTimestamp, true
	}
	return currentHourStart, endTimestamp, true
}

func currentHourStartTimestamp() int64 {
	now := common.GetTimestamp()
	return now - (now % 3600)
}

type topUsersSourceQuery struct {
	sql  string
	args []interface{}
}

type topUsersSettledSource struct {
	startTimestamp int64
	endTimestamp   int64
	useQuotaData   bool
}

func getTopUsersFromUsageUnion(startTimestamp int64, endTimestamp int64, modelName string, currentHourStart int64, limit int) ([]UserQuotaStat, error) {
	sources, err := buildTopUsersSourceQueries(startTimestamp, endTimestamp, modelName, currentHourStart)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, nil
	}

	sourceSQL := make([]string, 0, len(sources))
	args := make([]interface{}, 0)
	for _, source := range sources {
		sourceSQL = append(sourceSQL, source.sql)
		args = append(args, source.args...)
	}

	query := "SELECT usage_data.user_id, users.username, MAX(users.quota) as remaining_quota, " +
		"MAX(users.quota + users.used_quota) as total_quota, COALESCE(SUM(usage_data.quota), 0) as used_quota, " +
		topUserModelFamilySelect("LOWER(usage_data.model_name)", "usage_data.quota") +
		" FROM (" + strings.Join(sourceSQL, " UNION ALL ") + ") usage_data" +
		" INNER JOIN users ON users.id = usage_data.user_id" +
		" GROUP BY usage_data.user_id, users.username" +
		" ORDER BY used_quota DESC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	var results []UserQuotaStat
	err = DB.Raw(query, args...).Scan(&results).Error
	return results, err
}

func buildTopUsersSourceQueries(startTimestamp int64, endTimestamp int64, modelName string, currentHourStart int64) ([]topUsersSourceQuery, error) {
	sources := make([]topUsersSourceQuery, 0, 2)

	if settledSource, ok, err := chooseTopUsersSettledSource(startTimestamp, endTimestamp, modelName, currentHourStart); err != nil {
		return nil, err
	} else if ok {
		source, err := buildTopUsersSettledSourceQuery(settledSource, modelName)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}

	if currentStartTimestamp, currentEndTimestamp, ok := topUsersCurrentHourRange(startTimestamp, endTimestamp, currentHourStart); ok {
		source, err := buildTopUsersLogsSourceQuery(currentStartTimestamp, currentEndTimestamp, modelName)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}

	return sources, nil
}

func chooseTopUsersSettledSource(startTimestamp int64, endTimestamp int64, modelName string, currentHourStart int64) (topUsersSettledSource, bool, error) {
	settledStartTimestamp, settledEndTimestamp, ok := topUsersSettledRange(startTimestamp, endTimestamp, currentHourStart)
	if !ok {
		return topUsersSettledSource{}, false, nil
	}
	hasQuotaData, err := hasTopUsersQuotaData(settledStartTimestamp, settledEndTimestamp, modelName)
	if err != nil {
		return topUsersSettledSource{}, false, err
	}
	return topUsersSettledSource{
		startTimestamp: settledStartTimestamp,
		endTimestamp:   settledEndTimestamp,
		useQuotaData:   hasQuotaData,
	}, true, nil
}

func buildTopUsersSettledSourceQuery(source topUsersSettledSource, modelName string) (topUsersSourceQuery, error) {
	if source.useQuotaData {
		return buildTopUsersQuotaDataSourceQuery(source.startTimestamp, source.endTimestamp, modelName)
	}
	return buildTopUsersLogsSourceQuery(source.startTimestamp, source.endTimestamp, modelName)
}

func buildTopUsersQuotaDataSourceQuery(startTimestamp int64, endTimestamp int64, modelName string) (topUsersSourceQuery, error) {
	query := "SELECT quota_data.user_id, quota_data.model_name, quota_data.quota FROM quota_data WHERE 1 = 1"
	args := make([]interface{}, 0)
	if startTimestamp != 0 {
		query += " AND quota_data.created_at >= ?"
		args = append(args, startTimestamp)
	}
	if endTimestamp != 0 {
		query += " AND quota_data.created_at <= ?"
		args = append(args, endTimestamp)
	}
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return topUsersSourceQuery{}, err
		}
		query += " AND quota_data.model_name LIKE ? ESCAPE '!'"
		args = append(args, modelNamePattern)
	}
	return topUsersSourceQuery{sql: query, args: args}, nil
}

func buildTopUsersLogsSourceQuery(startTimestamp int64, endTimestamp int64, modelName string) (topUsersSourceQuery, error) {
	query := "SELECT logs.user_id, logs.model_name, logs.quota FROM logs WHERE logs.type = ?"
	args := []interface{}{LogTypeConsume}
	if startTimestamp != 0 {
		query += " AND logs.created_at >= ?"
		args = append(args, startTimestamp)
	}
	if endTimestamp != 0 {
		query += " AND logs.created_at <= ?"
		args = append(args, endTimestamp)
	}
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return topUsersSourceQuery{}, err
		}
		query += " AND logs.model_name LIKE ? ESCAPE '!'"
		args = append(args, modelNamePattern)
	}
	return topUsersSourceQuery{sql: query, args: args}, nil
}

func hasTopUsersQuotaData(startTimestamp int64, endTimestamp int64, modelName string) (bool, error) {
	tx := DB.Table("quota_data").Select("1 as matched")
	if startTimestamp != 0 {
		tx = tx.Where("quota_data.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("quota_data.created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return false, err
		}
		tx = tx.Where("quota_data.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}

	var row struct {
		Matched int `gorm:"column:matched"`
	}
	err := tx.Limit(1).Scan(&row).Error
	if err != nil {
		return false, err
	}
	return row.Matched == 1, nil
}

func getTopUsersMergedAcrossDB(startTimestamp int64, endTimestamp int64, modelName string, currentHourStart int64, limit int) ([]UserQuotaStat, error) {
	var merged []UserQuotaStat

	if settledSource, ok, err := chooseTopUsersSettledSource(startTimestamp, endTimestamp, modelName, currentHourStart); err != nil {
		return nil, err
	} else if ok {
		settledResults, err := getTopUsersFromSettledSource(settledSource, modelName)
		if err != nil {
			return nil, err
		}
		merged = mergeUserQuotaStats(merged, settledResults)
	}

	if currentStartTimestamp, currentEndTimestamp, ok := topUsersCurrentHourRange(startTimestamp, endTimestamp, currentHourStart); ok {
		currentResults, err := getTopUsersFromLogsAcrossDB(currentStartTimestamp, currentEndTimestamp, modelName, 0)
		if err != nil {
			return nil, err
		}
		merged = mergeUserQuotaStats(merged, currentResults)
	}

	if len(merged) > 0 {
		return limitUserQuotaStats(merged, limit), nil
	}

	return getTopUsersFromLogsAcrossDB(startTimestamp, endTimestamp, modelName, limit)
}

func getTopUsersFromSettledSource(source topUsersSettledSource, modelName string) ([]UserQuotaStat, error) {
	if source.useQuotaData {
		return getTopUsersFromQuotaData(source.startTimestamp, source.endTimestamp, modelName, 0)
	}
	return getTopUsersFromLogsAcrossDB(source.startTimestamp, source.endTimestamp, modelName, 0)
}

func getTopUsersFromLogsAcrossDB(startTimestamp int64, endTimestamp int64, modelName string, limit int) ([]UserQuotaStat, error) {
	results, err := getTopUsersUsageFromLogs(startTimestamp, endTimestamp, modelName, limit)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return results, nil
	}
	results, err = fillTopUsersInfo(results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func getTopUsersUsageFromLogs(startTimestamp int64, endTimestamp int64, modelName string, limit int) ([]UserQuotaStat, error) {
	selectSQL := "logs.user_id, COALESCE(SUM(logs.quota), 0) as used_quota, " +
		topUserModelFamilySelect("LOWER(logs.model_name)", "logs.quota")

	tx := LOG_DB.Table("logs").
		Select(selectSQL).
		Where("logs.type = ?", LogTypeConsume)

	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return nil, err
		}
		tx = tx.Where("logs.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}

	var results []UserQuotaStat
	tx = tx.Group("logs.user_id").
		Order("used_quota desc")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	err := tx.Scan(&results).Error
	return results, err
}

func fillTopUsersInfo(results []UserQuotaStat) ([]UserQuotaStat, error) {
	userIds := make([]int, 0, len(results))
	for _, result := range results {
		userIds = append(userIds, result.UserId)
	}

	var users []UserQuotaStat
	if err := DB.Table("users").
		Select("id as user_id, username, quota as remaining_quota, (quota + used_quota) as total_quota").
		Where("id IN ?", userIds).
		Scan(&users).Error; err != nil {
		return nil, err
	}

	userInfo := make(map[int]UserQuotaStat, len(users))
	for _, user := range users {
		userInfo[user.UserId] = user
	}
	filtered := make([]UserQuotaStat, 0, len(results))
	for i := range results {
		if user, ok := userInfo[results[i].UserId]; ok {
			results[i].Username = user.Username
			results[i].RemainingQuota = user.RemainingQuota
			results[i].TotalQuota = user.TotalQuota
			filtered = append(filtered, results[i])
		}
	}
	return filtered, nil
}

func mergeUserQuotaStats(base []UserQuotaStat, additions []UserQuotaStat) []UserQuotaStat {
	userStats := make(map[int]*UserQuotaStat, len(base)+len(additions))
	order := make([]int, 0, len(base)+len(additions))

	for _, stat := range base {
		statCopy := stat
		userStats[stat.UserId] = &statCopy
		order = append(order, stat.UserId)
	}
	for _, stat := range additions {
		existing, ok := userStats[stat.UserId]
		if !ok {
			statCopy := stat
			userStats[stat.UserId] = &statCopy
			order = append(order, stat.UserId)
			continue
		}
		existing.UsedQuota += stat.UsedQuota
		existing.GptQuota += stat.GptQuota
		existing.ClaudeQuota += stat.ClaudeQuota
		existing.DeepSeekQuota += stat.DeepSeekQuota
		existing.GeminiQuota += stat.GeminiQuota
		existing.QwenQuota += stat.QwenQuota
		existing.OtherQuota += stat.OtherQuota
		existing.Username = stat.Username
		existing.RemainingQuota = stat.RemainingQuota
		existing.TotalQuota = stat.TotalQuota
	}

	results := make([]UserQuotaStat, 0, len(userStats))
	for _, userId := range order {
		results = append(results, *userStats[userId])
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].UsedQuota == results[j].UsedQuota {
			return results[i].UserId < results[j].UserId
		}
		return results[i].UsedQuota > results[j].UsedQuota
	})
	return results
}

func limitUserQuotaStats(results []UserQuotaStat, limit int) []UserQuotaStat {
	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}

func topUserModelFamilySelect(modelExpr string, quotaExpr string) string {
	// modelExpr and quotaExpr must be internal SQL expressions, not user input.
	gptCond := modelFamilyLikeCondition(modelExpr, []string{"%gpt%", "o1%", "o3%", "o4%"})
	claudeCond := modelFamilyLikeCondition(modelExpr, []string{"%claude%"})
	deepSeekCond := modelFamilyLikeCondition(modelExpr, []string{"%deepseek%", "%deep-seek%"})
	geminiCond := modelFamilyLikeCondition(modelExpr, []string{"%gemini%"})
	qwenCond := modelFamilyLikeCondition(modelExpr, []string{"%qwen%", "%qwq%"})
	knownCond := strings.Join([]string{gptCond, claudeCond, deepSeekCond, geminiCond, qwenCond}, " OR ")

	return strings.Join([]string{
		modelFamilySumSelect(gptCond, quotaExpr, "gpt_quota"),
		modelFamilySumSelect(claudeCond, quotaExpr, "claude_quota"),
		modelFamilySumSelect(deepSeekCond, quotaExpr, "deepseek_quota"),
		modelFamilySumSelect(geminiCond, quotaExpr, "gemini_quota"),
		modelFamilySumSelect(qwenCond, quotaExpr, "qwen_quota"),
		modelFamilySumSelect("NOT ("+knownCond+")", quotaExpr, "other_quota"),
	}, ", ")
}

func modelFamilyLikeCondition(modelExpr string, patterns []string) string {
	parts := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		parts = append(parts, fmt.Sprintf("%s LIKE '%s'", modelExpr, pattern))
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

func modelFamilySumSelect(condition string, quotaExpr string, alias string) string {
	return fmt.Sprintf("COALESCE(SUM(CASE WHEN %s THEN %s ELSE 0 END), 0) AS %s", condition, quotaExpr, alias)
}

func getTopUsersFromQuotaData(startTimestamp int64, endTimestamp int64, modelName string, limit int) ([]UserQuotaStat, error) {
	selectSQL := "quota_data.user_id, users.username, MAX(users.quota) as remaining_quota, " +
		"MAX(users.quota + users.used_quota) as total_quota, COALESCE(SUM(quota_data.quota), 0) as used_quota, " +
		topUserModelFamilySelect("LOWER(quota_data.model_name)", "quota_data.quota")

	tx := DB.Table("quota_data").
		Select(selectSQL).
		Joins("inner join users on users.id = quota_data.user_id")

	if startTimestamp != 0 {
		tx = tx.Where("quota_data.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("quota_data.created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return nil, err
		}
		tx = tx.Where("quota_data.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}

	var results []UserQuotaStat
	tx = tx.Group("quota_data.user_id, users.username").
		Order("used_quota desc")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	err := tx.Scan(&results).Error
	return results, err
}

func getTopUsersFromLogs(startTimestamp int64, endTimestamp int64, modelName string, limit int) ([]UserQuotaStat, error) {
	selectSQL := "logs.user_id, users.username, MAX(users.quota) as remaining_quota, " +
		"MAX(users.quota + users.used_quota) as total_quota, COALESCE(SUM(logs.quota), 0) as used_quota, " +
		topUserModelFamilySelect("LOWER(logs.model_name)", "logs.quota")

	tx := LOG_DB.Table("logs").
		Select(selectSQL).
		Joins("inner join users on users.id = logs.user_id").
		Where("logs.type = ?", LogTypeConsume)

	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return nil, err
		}
		tx = tx.Where("logs.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	var results []UserQuotaStat
	tx = tx.Group("logs.user_id, users.username").
		Order("used_quota desc")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	err := tx.Scan(&results).Error

	return results, err
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat, err error) {
	tx := LOG_DB.Table("logs").Select("sum(quota) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")

	if username != "" {
		tx = tx.Where("username = ?", username)
		rpmTpmQuery = rpmTpmQuery.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
		rpmTpmQuery = rpmTpmQuery.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return stat, err
		}
		tx = tx.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
		rpmTpmQuery = rpmTpmQuery.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", group)
	}

	tx = tx.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := rpmTpmQuery.Scan(&stat).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}

type UserRechargeStat struct {
	UserId            int    `json:"user_id"`
	Username          string `json:"username"`
	RemainingQuota    int    `json:"remaining_quota"`
	TotalQuota        int    `json:"total_quota"`
	UsedQuota         int    `json:"used_quota"`
	GptQuota          int    `json:"gpt_quota"`
	ClaudeQuota       int    `json:"claude_quota"`
	DeepSeekQuota     int    `json:"deepseek_quota"`
	GeminiQuota       int    `json:"gemini_quota"`
	QwenQuota         int    `json:"qwen_quota"`
	OtherQuota        int    `json:"other_quota"`
	TotalCount        int    `json:"total_count"`
	AutoRechargeCount int    `json:"auto_recharge_count"`
	TempQuotaCount    int    `json:"temp_quota_count"`
}

type rechargeUsageStat struct {
	UserId        int `gorm:"column:user_id"`
	UsedQuota     int `gorm:"column:used_quota"`
	GptQuota      int `gorm:"column:gpt_quota"`
	ClaudeQuota   int `gorm:"column:claude_quota"`
	DeepSeekQuota int `gorm:"column:deepseek_quota"`
	GeminiQuota   int `gorm:"column:gemini_quota"`
	QwenQuota     int `gorm:"column:qwen_quota"`
	OtherQuota    int `gorm:"column:other_quota"`
}

func GetRechargeLeaderboard(limit int) ([]UserRechargeStat, error) {
	if limit <= 0 {
		limit = 10
	} else if limit > 30 {
		limit = 30
	}

	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location()).Unix()

	type statRow struct {
		UserId int
		Count  int
	}

	autoMap := make(map[int]int)
	tempMap := make(map[int]int)
	totalMap := make(map[int]int)
	candidateUserSet := make(map[int]struct{})

	var autoRows []statRow
	err := LOG_DB.Table("logs").
		Select("logs.user_id, COUNT(*) as count").
		Where("logs.created_at >= ?", weekStart).
		Where("logs.type = ? AND logs.content LIKE ?", LogTypeSystem, "系统自动赠送%").
		Group("logs.user_id").
		Scan(&autoRows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range autoRows {
		autoMap[row.UserId] = row.Count
		totalMap[row.UserId] += row.Count
		candidateUserSet[row.UserId] = struct{}{}
	}

	var tempRows []statRow
	err = LOG_DB.Table("logs").
		Select("logs.user_id, COUNT(*) as count").
		Where("logs.created_at >= ?", weekStart).
		Where("logs.type = ? AND logs.content LIKE ?", LogTypeManage, "%临时额度").
		Group("logs.user_id").
		Scan(&tempRows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range tempRows {
		tempMap[row.UserId] = row.Count
		totalMap[row.UserId] += row.Count
		candidateUserSet[row.UserId] = struct{}{}
	}

	if len(totalMap) == 0 {
		return []UserRechargeStat{}, nil
	}

	type candidate struct {
		UserId     int
		TotalCount int
	}
	candidates := make([]candidate, 0, len(totalMap))
	for userId := range candidateUserSet {
		candidates = append(candidates, candidate{UserId: userId, TotalCount: totalMap[userId]})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].TotalCount == candidates[j].TotalCount {
			return candidates[i].UserId < candidates[j].UserId
		}
		return candidates[i].TotalCount > candidates[j].TotalCount
	})

	type userInfo struct {
		Id             int
		Username       string
		RemainingQuota int
		TotalQuota     int
	}

	selectedCandidates := make([]candidate, 0, limit)
	userMap := make(map[int]userInfo, limit)
	batchSize := limit * 4
	if batchSize < 64 {
		batchSize = 64
	}

	for start := 0; start < len(candidates) && len(selectedCandidates) < limit; {
		end := start + 1
		for end < len(candidates) && candidates[end].TotalCount == candidates[start].TotalCount {
			end++
		}

		for batchStart := start; batchStart < end; batchStart += batchSize {
			batchEnd := batchStart + batchSize
			if batchEnd > end {
				batchEnd = end
			}
			batch := candidates[batchStart:batchEnd]

			batchUserIds := make([]int, 0, len(batch))
			for _, c := range batch {
				batchUserIds = append(batchUserIds, c.UserId)
			}

			var users []userInfo
			err = LOG_DB.Table("users").
				Select("id, username, quota as remaining_quota, (quota + used_quota) as total_quota").
				Where("id IN ?", batchUserIds).
				Scan(&users).Error
			if err != nil {
				return nil, err
			}

			batchUserMap := make(map[int]struct{}, len(users))
			for _, u := range users {
				batchUserMap[u.Id] = struct{}{}
				userMap[u.Id] = u
			}

			for _, c := range batch {
				if _, ok := batchUserMap[c.UserId]; ok {
					selectedCandidates = append(selectedCandidates, c)
				}
			}
		}
		start = end
	}

	if len(selectedCandidates) == 0 {
		return []UserRechargeStat{}, nil
	}

	userIds := make([]int, 0, len(selectedCandidates))
	for _, c := range selectedCandidates {
		userIds = append(userIds, c.UserId)
	}

	currentHourStart := currentHourStartTimestamp()
	usedMap, err := getRechargeLeaderboardUsageStats(weekStart, userIds, currentHourStart)
	if err != nil {
		return nil, err
	}

	results := make([]UserRechargeStat, 0, len(selectedCandidates))
	for _, c := range selectedCandidates {
		u, ok := userMap[c.UserId]
		if !ok {
			continue
		}
		results = append(results, UserRechargeStat{
			UserId:            c.UserId,
			Username:          u.Username,
			RemainingQuota:    u.RemainingQuota,
			TotalQuota:        u.TotalQuota,
			UsedQuota:         usedMap[c.UserId].UsedQuota,
			GptQuota:          usedMap[c.UserId].GptQuota,
			ClaudeQuota:       usedMap[c.UserId].ClaudeQuota,
			DeepSeekQuota:     usedMap[c.UserId].DeepSeekQuota,
			GeminiQuota:       usedMap[c.UserId].GeminiQuota,
			QwenQuota:         usedMap[c.UserId].QwenQuota,
			OtherQuota:        usedMap[c.UserId].OtherQuota,
			TotalCount:        c.TotalCount,
			AutoRechargeCount: autoMap[c.UserId],
			TempQuotaCount:    tempMap[c.UserId],
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].TotalCount == results[j].TotalCount {
			if results[i].UsedQuota == results[j].UsedQuota {
				return results[i].UserId < results[j].UserId
			}
			return results[i].UsedQuota > results[j].UsedQuota
		}
		return results[i].TotalCount > results[j].TotalCount
	})
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func getRechargeLeaderboardUsageStats(weekStart int64, userIds []int, currentHourStart int64) (map[int]rechargeUsageStat, error) {
	usedMap := make(map[int]rechargeUsageStat, len(userIds))
	if len(userIds) == 0 {
		return usedMap, nil
	}

	if settledStart, settledEnd, ok := topUsersSettledRange(weekStart, 0, currentHourStart); ok {
		settledStats, err := getRechargeUsageStatsFromQuotaData(settledStart, settledEnd, userIds)
		if err != nil {
			return nil, err
		}
		mergeRechargeUsageStats(usedMap, settledStats)

		missingUserIds := rechargeUsageMissingUserIds(userIds, settledStats)
		if len(missingUserIds) > 0 {
			missingStats, err := getRechargeUsageStatsFromLogs(settledStart, settledEnd, missingUserIds)
			if err != nil {
				return nil, err
			}
			mergeRechargeUsageStats(usedMap, missingStats)
		}
	}

	if currentStart, currentEnd, ok := topUsersCurrentHourRange(weekStart, 0, currentHourStart); ok {
		currentStats, err := getRechargeUsageStatsFromLogs(currentStart, currentEnd, userIds)
		if err != nil {
			return nil, err
		}
		mergeRechargeUsageStats(usedMap, currentStats)
	}

	return usedMap, nil
}

func getRechargeUsageStatsFromQuotaData(startTimestamp int64, endTimestamp int64, userIds []int) ([]rechargeUsageStat, error) {
	selectSQL := "quota_data.user_id, COALESCE(SUM(quota_data.quota), 0) as used_quota, " +
		topUserModelFamilySelect("LOWER(quota_data.model_name)", "quota_data.quota")

	tx := DB.Table("quota_data").
		Select(selectSQL).
		Where("quota_data.user_id IN ?", userIds)
	if startTimestamp != 0 {
		tx = tx.Where("quota_data.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("quota_data.created_at <= ?", endTimestamp)
	}

	var results []rechargeUsageStat
	err := tx.Group("quota_data.user_id").Scan(&results).Error
	return results, err
}

func getRechargeUsageStatsFromLogs(startTimestamp int64, endTimestamp int64, userIds []int) ([]rechargeUsageStat, error) {
	selectSQL := "logs.user_id, COALESCE(SUM(logs.quota), 0) as used_quota, " +
		topUserModelFamilySelect("LOWER(logs.model_name)", "logs.quota")

	tx := LOG_DB.Table("logs").
		Select(selectSQL).
		Where("logs.type = ? AND logs.user_id IN ?", LogTypeConsume, userIds)
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}

	var results []rechargeUsageStat
	err := tx.Group("logs.user_id").Scan(&results).Error
	return results, err
}

func rechargeUsageMissingUserIds(userIds []int, stats []rechargeUsageStat) []int {
	statUserIds := make(map[int]struct{}, len(stats))
	for _, stat := range stats {
		statUserIds[stat.UserId] = struct{}{}
	}

	missingUserIds := make([]int, 0, len(userIds))
	for _, userId := range userIds {
		if _, ok := statUserIds[userId]; !ok {
			missingUserIds = append(missingUserIds, userId)
		}
	}
	return missingUserIds
}

func mergeRechargeUsageStats(base map[int]rechargeUsageStat, additions []rechargeUsageStat) {
	for _, stat := range additions {
		existing := base[stat.UserId]
		existing.UserId = stat.UserId
		existing.UsedQuota += stat.UsedQuota
		existing.GptQuota += stat.GptQuota
		existing.ClaudeQuota += stat.ClaudeQuota
		existing.DeepSeekQuota += stat.DeepSeekQuota
		existing.GeminiQuota += stat.GeminiQuota
		existing.QwenQuota += stat.QwenQuota
		existing.OtherQuota += stat.OtherQuota
		base[stat.UserId] = existing
	}
}
