package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DingTalkNotificationEventNewUserQuotaExhausted = "new_user_quota_exhausted"
	DingTalkNotificationEventUserRegistered        = "user_registered"
	DingTalkNotificationEventQuotaPoolJoined       = "quota_pool_joined"
	DingTalkNotificationEventSensitiveWordDetected = "sensitive_word_detected"
	DingTalkNotificationEventAnnouncementGroup     = "announcement_group"
	DingTalkNotificationEventTest                  = "test"

	DingTalkNotificationStatusPending   = "pending"
	DingTalkNotificationStatusRunning   = "running"
	DingTalkNotificationStatusSucceeded = "succeeded"
	DingTalkNotificationStatusFailed    = "failed"
	DingTalkNotificationStatusSkipped   = "skipped"
)

const dingTalkNotificationHourlyLimitError = "DingTalk notification hourly limit exceeded"

var dingTalkNotificationRateLimitMutex sync.Mutex

// DingTalkNotification 记录钉钉消息的投递结果，并通过 DedupeKey 防止同一业务事件重复发送。
type DingTalkNotification struct {
	Id          int64  `json:"id"`
	EventType   string `json:"event_type" gorm:"type:varchar(64);index"`
	DedupeKey   string `json:"dedupe_key" gorm:"type:varchar(191);uniqueIndex"`
	UserId      int    `json:"user_id" gorm:"index"`
	Username    string `json:"username" gorm:"type:varchar(64);index"`
	Recipient   string `json:"recipient" gorm:"type:varchar(128);index"`
	Title       string `json:"title" gorm:"type:varchar(255)"`
	Content     string `json:"content" gorm:"type:text"`
	Status      string `json:"status" gorm:"type:varchar(32);index"`
	Error       string `json:"error" gorm:"type:text"`
	Metadata    string `json:"metadata" gorm:"type:text"`
	SentAt      int64  `json:"sent_at" gorm:"bigint"`
	ScheduledAt int64  `json:"scheduled_at" gorm:"bigint;index"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint;autoCreateTime;index"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

func UpsertPendingAnnouncementGroupNotification(notification *DingTalkNotification) (bool, error) {
	if notification == nil || notification.EventType != DingTalkNotificationEventAnnouncementGroup || strings.TrimSpace(notification.DedupeKey) == "" || notification.ScheduledAt <= 0 {
		return false, fmt.Errorf("invalid announcement group notification")
	}
	var existing DingTalkNotification
	err := DB.Where("dedupe_key = ?", notification.DedupeKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		notification.Status = DingTalkNotificationStatusPending
		return CreateDingTalkNotification(notification)
	}
	if err != nil {
		return false, err
	}
	if existing.Status != DingTalkNotificationStatusPending {
		return false, nil
	}
	_, err = UpdatePendingAnnouncementGroupNotification(notification)
	return false, err
}

func UpdatePendingAnnouncementGroupNotification(notification *DingTalkNotification) (bool, error) {
	if notification == nil || strings.TrimSpace(notification.DedupeKey) == "" {
		return false, fmt.Errorf("invalid announcement group notification")
	}
	result := DB.Model(&DingTalkNotification{}).Where("dedupe_key = ? AND event_type = ? AND status = ?", notification.DedupeKey, DingTalkNotificationEventAnnouncementGroup, DingTalkNotificationStatusPending).Updates(map[string]any{
		"recipient": notification.Recipient, "title": notification.Title,
		"content": notification.Content, "metadata": notification.Metadata,
		"scheduled_at": notification.ScheduledAt, "error": "",
	})
	return result.RowsAffected == 1, result.Error
}

func CancelPendingAnnouncementGroupNotification(dedupeKey, reason string) error {
	return DB.Model(&DingTalkNotification{}).
		Where("dedupe_key = ? AND event_type = ? AND status = ?", dedupeKey, DingTalkNotificationEventAnnouncementGroup, DingTalkNotificationStatusPending).
		Updates(map[string]any{"status": DingTalkNotificationStatusSkipped, "error": reason}).Error
}

func ListDueAnnouncementGroupNotifications(now int64, limit int) ([]DingTalkNotification, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	var records []DingTalkNotification
	err := DB.Where("event_type = ? AND status = ? AND scheduled_at <= ?", DingTalkNotificationEventAnnouncementGroup, DingTalkNotificationStatusPending, now).
		Order("scheduled_at ASC, id ASC").Limit(limit).Find(&records).Error
	return records, err
}

func HasDueAnnouncementGroupNotifications(now int64) (bool, error) {
	var count int64
	err := DB.Model(&DingTalkNotification{}).
		Where("event_type = ? AND status = ? AND scheduled_at <= ?", DingTalkNotificationEventAnnouncementGroup, DingTalkNotificationStatusPending, now).
		Limit(1).Count(&count).Error
	return count > 0, err
}

func ClaimDueAnnouncementGroupNotifications(now int64, limit int) ([]DingTalkNotification, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	var claimed []DingTalkNotification
	err := DB.Transaction(func(tx *gorm.DB) error {
		var candidates []DingTalkNotification
		if err := lockForUpdate(tx).
			Where("event_type = ? AND status = ? AND scheduled_at <= ?", DingTalkNotificationEventAnnouncementGroup, DingTalkNotificationStatusPending, now).
			Order("scheduled_at ASC, id ASC").Limit(limit).Find(&candidates).Error; err != nil {
			return err
		}
		if len(candidates) == 0 {
			return nil
		}
		ids := make([]int64, 0, len(candidates))
		for _, candidate := range candidates {
			ids = append(ids, candidate.Id)
		}
		if err := tx.Model(&DingTalkNotification{}).
			Where("id IN ? AND status = ?", ids, DingTalkNotificationStatusPending).
			Update("status", DingTalkNotificationStatusRunning).Error; err != nil {
			return err
		}
		return tx.Where("id IN ? AND status = ?", ids, DingTalkNotificationStatusRunning).
			Order("scheduled_at ASC, id ASC").Find(&claimed).Error
	})
	return claimed, err
}

func FailStaleRunningAnnouncementGroupNotifications(cutoff int64) error {
	return DB.Model(&DingTalkNotification{}).
		Where("event_type = ? AND status = ? AND updated_at < ?", DingTalkNotificationEventAnnouncementGroup, DingTalkNotificationStatusRunning, cutoff).
		Updates(map[string]any{"status": DingTalkNotificationStatusFailed, "error": "announcement group delivery interrupted"}).Error
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

// CreateHourlyRateLimitedDingTalkNotification 在同一用户行锁内完成去重和小时额度预留。
// 进程锁补足 SQLite 不支持 FOR UPDATE 的限制；MySQL/PostgreSQL 仍由数据库行锁保证多实例串行。
func CreateHourlyRateLimitedDingTalkNotification(notification *DingTalkNotification, windowStart, windowEnd int64, maxCount int) (created, allowed bool, err error) {
	if notification == nil || notification.UserId <= 0 || windowStart >= windowEnd || maxCount <= 0 {
		return false, false, fmt.Errorf("invalid DingTalk notification hourly limit parameters")
	}

	dingTalkNotificationRateLimitMutex.Lock()
	defer dingTalkNotificationRateLimitMutex.Unlock()

	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id").First(&user, notification.UserId).Error; err != nil {
			return err
		}

		var duplicateCount int64
		if err := tx.Model(&DingTalkNotification{}).
			Where("dedupe_key = ?", notification.DedupeKey).
			Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			return nil
		}

		var hourlyCount int64
		if err := tx.Model(&DingTalkNotification{}).
			Where("event_type = ? AND user_id = ? AND created_at >= ? AND created_at < ?", notification.EventType, notification.UserId, windowStart, windowEnd).
			Count(&hourlyCount).Error; err != nil {
			return err
		}
		allowed = hourlyCount < int64(maxCount)
		if !allowed {
			notification.Status = DingTalkNotificationStatusSkipped
			notification.Error = dingTalkNotificationHourlyLimitError
		}
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "dedupe_key"}},
			DoNothing: true,
		}).Create(notification)
		created = result.RowsAffected == 1
		if !created {
			allowed = false
		}
		return result.Error
	})
	return created, allowed, err
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
