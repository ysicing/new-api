package model

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
	UserId           int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1;index:idx_logs_type_created_at_user_id,priority:3"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:2;index:idx_created_at_type;index:idx_logs_type_created_at_user_id,priority:2"`
	Type             int    `json:"type" gorm:"index:idx_created_at_type;index:idx_logs_type_created_at_user_id,priority:1"`
	Content          string `json:"content"`
	Username         string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
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

// CountAutoRechargeLogs 统计用户在指定时间范围内的自动充值次数
func CountAutoRechargeLogs(userId int, sinceTimestamp int64) (int64, error) {
	var count int64
	err := LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ? AND content LIKE ? AND created_at >= ?",
			userId, LogTypeSystem, "系统自动赠送%", sinceTimestamp).
		Count(&count).Error
	return count, err
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
		if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
			return logs, total, err
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

type UserQuotaStat struct {
	Username       string `json:"username"`
	RemainingQuota int    `json:"remaining_quota"`
	TotalQuota     int    `json:"total_quota"`
	UsedQuota      int    `json:"used_quota"`
}

func GetTopUsers(startTimestamp int64, endTimestamp int64, modelName string, channel int, group string, limit int) ([]UserQuotaStat, error) {
	// 限制limit范围
	if limit <= 0 {
		limit = 10
	} else if limit > 30 {
		limit = 30
	}

	// 限制查询时间范围最多30天
	if startTimestamp > 0 && endTimestamp > 0 {
		maxDuration := int64(30 * 24 * 60 * 60) // 30天
		if endTimestamp-startTimestamp > maxDuration {
			startTimestamp = endTimestamp - maxDuration
		}
	}

	tx := LOG_DB.Table("logs").
		Select("users.username, MAX(users.quota) as remaining_quota, MAX(users.quota + users.used_quota) as total_quota, sum(logs.quota) as used_quota").
		Joins("inner join users on users.id = logs.user_id").
		Where("logs.type = ?", LogTypeConsume)

	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("logs.model_name like ?", modelName)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}

	var results []UserQuotaStat
	err := tx.Group("logs.user_id").
		Order("used_quota desc").
		Limit(limit).
		Scan(&results).Error

	return results, err
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

// UserRechargeStat 用户充值排行统计
type UserRechargeStat struct {
	UserId            int    `json:"user_id"`
	Username          string `json:"username"`
	RemainingQuota    int    `json:"remaining_quota"`
	TotalQuota        int    `json:"total_quota"`
	UsedQuota         int    `json:"used_quota"`
	TotalCount        int    `json:"total_count"`
	AutoRechargeCount int    `json:"auto_recharge_count"`
	TempQuotaCount    int    `json:"temp_quota_count"`
}

// GetRechargeLeaderboard 查询本周自动充值次数和临时额度赠送次数排行
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

	for start := 0; start < len(candidates) && len(selectedCandidates) < limit; start += batchSize {
		end := start + batchSize
		if end > len(candidates) {
			end = len(candidates)
		}
		batch := candidates[start:end]

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
				if len(selectedCandidates) >= limit {
					break
				}
			}
		}
	}

	if len(selectedCandidates) == 0 {
		return []UserRechargeStat{}, nil
	}

	userIds := make([]int, 0, len(selectedCandidates))
	for _, c := range selectedCandidates {
		userIds = append(userIds, c.UserId)
	}

	type usedStat struct {
		UserId    int
		UsedQuota int
	}
	var usedStats []usedStat
	err = LOG_DB.Table("logs").
		Select("logs.user_id, COALESCE(SUM(logs.quota), 0) as used_quota").
		Where("logs.type = ? AND logs.created_at >= ? AND logs.user_id IN ?", LogTypeConsume, weekStart, userIds).
		Group("logs.user_id").
		Scan(&usedStats).Error
	if err != nil {
		return nil, err
	}

	usedMap := make(map[int]int, len(usedStats))
	for _, s := range usedStats {
		usedMap[s.UserId] = s.UsedQuota
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
			UsedQuota:         usedMap[c.UserId],
			TotalCount:        c.TotalCount,
			AutoRechargeCount: autoMap[c.UserId],
			TempQuotaCount:    tempMap[c.UserId],
		})
	}

	return results, nil
}
