package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/bytedance/gopkg/util/gopool"
)

const sensitiveWordNotificationHourlyLimit = 3

func sensitiveWordNotificationContent(lang string, detectedAt time.Time) string {
	parts := []string{i18n.Translate(lang, i18n.MsgNotifySensitiveWordIntro)}
	if usageLogsURL := usageErrorLogsURL(detectedAt); usageLogsURL != "" {
		parts = append(parts, fmt.Sprintf("[%s](%s)", i18n.Translate(lang, i18n.MsgNotifyViewUsageLogs), usageLogsURL))
	}
	parts = append(parts, i18n.Translate(lang, i18n.MsgNotifySensitiveWordGuidance))
	contactMessage := setting.NormalizeSensitiveWordContactMessage(setting.SensitiveWordContactMessage)
	if contactMessage != "" {
		parts = append(parts, contactMessage)
	}
	return strings.Join(parts, "\n\n")
}

func usageErrorLogsURL(detectedAt time.Time) string {
	baseURL, err := url.Parse(strings.TrimSpace(system_setting.ServerAddress))
	if err != nil || baseURL.Host == "" || baseURL.User != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return ""
	}
	dayStart := time.Date(detectedAt.Year(), detectedAt.Month(), detectedAt.Day(), 0, 0, 0, 0, detectedAt.Location())
	query := url.Values{}
	query.Set("startTime", strconv.FormatInt(dayStart.UnixMilli(), 10))
	query.Set("endTime", strconv.FormatInt(detectedAt.UnixMilli(), 10))
	query.Set("type", fmt.Sprintf(`["%d"]`, model.LogTypeError))
	query.Set("page", "1")
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/usage-logs/common"
	baseURL.RawQuery = query.Encode()
	baseURL.Fragment = ""
	return baseURL.String()
}

// NotifySensitiveWordsDetected 异步发送敏感词审查通知，避免钉钉网络请求影响原始拒绝响应。
func NotifySensitiveWordsDetected(userId int, words []string) {
	if userId <= 0 || len(words) == 0 {
		return
	}
	detectedAt := time.Now()
	words = append([]string(nil), words...)
	gopool.Go(func() {
		if err := notifySensitiveWordsDetectedAt(userId, words, detectedAt); err != nil {
			common.SysError(fmt.Sprintf("failed to send sensitive-word DingTalk notification to user %d: %s", userId, err.Error()))
		}
	})
}

func notifySensitiveWordsDetectedAt(userId int, words []string, detectedAt time.Time) error {
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(words))
	for _, word := range words {
		normalizedWord := strings.ToLower(strings.TrimSpace(word))
		if normalizedWord == "" {
			continue
		}
		if _, exists := seen[normalizedWord]; exists {
			continue
		}
		seen[normalizedWord] = struct{}{}
		wordDigest := sha256.Sum256([]byte(normalizedWord))
		hourStart := time.Date(detectedAt.Year(), detectedAt.Month(), detectedAt.Day(), detectedAt.Hour(), 0, 0, 0, detectedAt.Location())
		created, allowed, dispatchErr := dispatchHourlyRateLimitedDingTalkNotification(context.Background(), DingTalkNotificationRequest{
			EventType: model.DingTalkNotificationEventSensitiveWordDetected,
			DedupeKey: fmt.Sprintf("%s:%d:%d:%x", model.DingTalkNotificationEventSensitiveWordDetected, user.Id, hourStart.Unix(), wordDigest),
			UserId:    user.Id, UserEmail: user.Email,
			Title:    i18n.Translate(user.GetSetting().Language, i18n.MsgNotifySensitiveWordTitle),
			Content:  sensitiveWordNotificationContent(user.GetSetting().Language, detectedAt),
			Metadata: map[string]any{"window_start": hourStart.Unix(), "window_end": hourStart.Add(time.Hour).Unix()},
		}, detectedAt, sensitiveWordNotificationHourlyLimit)
		if dispatchErr != nil {
			return dispatchErr
		}
		// 第一条超额记录已经说明限频原因，后续敏感词无需继续占用数据库记录。
		if created && !allowed {
			break
		}
	}
	return nil
}
