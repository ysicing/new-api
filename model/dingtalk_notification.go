package model

import (
	"strings"

	"gorm.io/gorm/clause"
)

const (
	DingTalkNotificationEventNewUserQuotaExhausted = "new_user_quota_exhausted"
	DingTalkNotificationEventTest                  = "test"

	DingTalkNotificationStatusPending   = "pending"
	DingTalkNotificationStatusSucceeded = "succeeded"
	DingTalkNotificationStatusFailed    = "failed"
	DingTalkNotificationStatusSkipped   = "skipped"
)

// DingTalkNotification 记录钉钉消息的投递结果，并通过 DedupeKey 防止同一业务事件重复发送。
type DingTalkNotification struct {
	Id        int64  `json:"id"`
	EventType string `json:"event_type" gorm:"type:varchar(64);index"`
	DedupeKey string `json:"dedupe_key" gorm:"type:varchar(191);uniqueIndex"`
	UserId    int    `json:"user_id" gorm:"index"`
	Username  string `json:"username" gorm:"type:varchar(64);index"`
	Recipient string `json:"recipient" gorm:"type:varchar(128);index"`
	Title     string `json:"title" gorm:"type:varchar(255)"`
	Content   string `json:"content" gorm:"type:text"`
	Status    string `json:"status" gorm:"type:varchar(32);index"`
	Error     string `json:"error" gorm:"type:text"`
	Metadata  string `json:"metadata" gorm:"type:text"`
	SentAt    int64  `json:"sent_at" gorm:"bigint"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;autoCreateTime;index"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

type DingTalkNotificationQuery struct {
	EventType      string
	Status         string
	Keyword        string
	StartTimestamp int64
	EndTimestamp   int64
	StartIdx       int
	PageSize       int
}

// CreateDingTalkNotification 创建通知记录。DedupeKey 已存在时返回 created=false。
func CreateDingTalkNotification(notification *DingTalkNotification) (created bool, err error) {
	result := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dedupe_key"}},
		DoNothing: true,
	}).Create(notification)
	return result.RowsAffected == 1, result.Error
}

func UpdateDingTalkNotificationResult(id int64, status, errorText string, sentAt int64) error {
	return DB.Model(&DingTalkNotification{}).Where("id = ?", id).Updates(map[string]any{
		"status": status, "error": errorText, "sent_at": sentAt,
	}).Error
}

func ListDingTalkNotifications(query DingTalkNotificationQuery) ([]DingTalkNotification, int64, error) {
	tx := DB.Model(&DingTalkNotification{})
	if query.EventType != "" {
		tx = tx.Where("event_type = ?", query.EventType)
	}
	if query.Status != "" {
		tx = tx.Where("status = ?", query.Status)
	}
	if query.StartTimestamp > 0 {
		tx = tx.Where("created_at >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp > 0 {
		tx = tx.Where("created_at <= ?", query.EndTimestamp)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(keyword))
		like := "%" + escaped + "%"
		tx = tx.Where("(LOWER(username) LIKE ? ESCAPE '!' OR LOWER(recipient) LIKE ? ESCAPE '!')", like, like)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	} else if pageSize > 100 {
		pageSize = 100
	}
	startIdx := query.StartIdx
	if startIdx < 0 {
		startIdx = 0
	}
	var items []DingTalkNotification
	if err := tx.Order("id DESC").Offset(startIdx).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
