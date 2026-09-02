package service

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"golang.org/x/net/html"
)

const announcementGroupDeliveryBatchSize = 100

type announcementNotification struct {
	Id          int    `json:"id"`
	Content     string `json:"content"`
	PublishDate string `json:"publishDate"`
	Type        string `json:"type"`
	Extra       string `json:"extra"`
}

type announcementGroupDeliveryResult struct {
	Attempted int `json:"attempted"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

var sendAnnouncementGroupMessage = func(ctx context.Context, settings system_setting.DingTalkSettings, conversationID, title, content string) error {
	return defaultDingTalkNotifier.SendGroup(ctx, settings, conversationID, title, content)
}

func SendDingTalkAnnouncementGroupTestMessage(ctx context.Context) error {
	settings := *system_setting.GetDingTalkSettings()
	if !settings.IsRobotConfigured() {
		return ErrDingTalkNotConfigured
	}
	conversationID := strings.TrimSpace(settings.AnnouncementGroupOpenConversationId)
	if conversationID == "" {
		return ErrDingTalkGroupNotConfigured
	}
	content := "公告群通知配置成功。\n\n发送时间：" + time.Now().Format(time.RFC3339)
	if err := sendAnnouncementGroupMessage(ctx, settings, conversationID, "系统公告测试", content); err != nil {
		return fmt.Errorf("%w: %v", ErrDingTalkSendFailed, err)
	}
	return nil
}

func parseAnnouncementNotifications(raw string) ([]announcementNotification, error) {
	if strings.TrimSpace(raw) == "" {
		return []announcementNotification{}, nil
	}
	var announcements []announcementNotification
	if err := common.UnmarshalJsonStr(raw, &announcements); err != nil {
		return nil, err
	}
	return announcements, nil
}

func announcementGroupDedupeKey(id int) string {
	return fmt.Sprintf("announcement_group:%d", id)
}

func announcementMarkdownContent(announcement announcementNotification, publishAt time.Time) string {
	parts := []string{stripAnnouncementHTML(announcement.Content)}
	if extra := stripAnnouncementHTML(announcement.Extra); extra != "" {
		parts = append(parts, extra)
	}
	parts = append(parts, "发布时间："+publishAt.Format("2006-01-02 15:04"))
	return strings.Join(parts, "\n\n")
}

func stripAnnouncementHTML(raw string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(raw))
	var builder strings.Builder
	linkStack := make([]string, 0, 2)
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			if tokenizer.Err() != nil && tokenizer.Err() != io.EOF {
				return strings.TrimSpace(raw)
			}
			lines := strings.Split(builder.String(), "\n")
			cleaned := make([]string, 0, len(lines))
			for _, line := range lines {
				if line = strings.TrimSpace(line); line != "" {
					cleaned = append(cleaned, line)
				}
			}
			return strings.Join(cleaned, "\n")
		case html.TextToken:
			builder.Write(tokenizer.Text())
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			switch token.Data {
			case "a":
				href := announcementHTMLAttribute(token, "href")
				if !isSafeAnnouncementURL(href) {
					href = ""
				} else {
					builder.WriteByte('[')
				}
				linkStack = append(linkStack, href)
			case "img":
				src := announcementHTMLAttribute(token, "src")
				if isSafeAnnouncementURL(src) {
					builder.WriteString("![" + announcementHTMLAttribute(token, "alt") + "](" + src + ")")
				}
			case "br", "p", "div", "li":
				builder.WriteByte('\n')
			}
		case html.EndTagToken:
			token := tokenizer.Token()
			if token.Data == "a" && len(linkStack) > 0 {
				href := linkStack[len(linkStack)-1]
				linkStack = linkStack[:len(linkStack)-1]
				if href != "" {
					builder.WriteString("](" + href + ")")
				}
			}
			switch token.Data {
			case "p", "div", "li":
				builder.WriteByte('\n')
			}
		}
	}
}

func announcementHTMLAttribute(token html.Token, key string) string {
	for _, attribute := range token.Attr {
		if attribute.Key == key {
			return strings.TrimSpace(attribute.Val)
		}
	}
	return ""
}

func isSafeAnnouncementURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func SyncAnnouncementGroupNotifications(previousRaw, currentRaw string, operatorId int, operatorName string, now time.Time) error {
	previous, err := parseAnnouncementNotifications(previousRaw)
	if err != nil {
		return err
	}
	current, err := parseAnnouncementNotifications(currentRaw)
	if err != nil {
		return err
	}
	previousById := make(map[int]announcementNotification, len(previous))
	currentById := make(map[int]announcementNotification, len(current))
	for _, announcement := range previous {
		previousById[announcement.Id] = announcement
	}
	for _, announcement := range current {
		currentById[announcement.Id] = announcement
	}
	settings := *system_setting.GetDingTalkSettings()
	conversationID := strings.TrimSpace(settings.AnnouncementGroupOpenConversationId)
	hasDue := false
	for id, announcement := range currentById {
		publishAt, err := time.Parse(time.RFC3339, announcement.PublishDate)
		if err != nil || id <= 0 {
			continue
		}
		// 前端以 UTC 保存 RFC3339 时间；正文展示使用服务本地时区，调度仍使用原始 Unix 时间戳。
		displayPublishAt := publishAt.In(now.Location())
		_, existed := previousById[id]
		if !existed && conversationID == "" {
			metadata, _ := common.Marshal(map[string]any{"announcement_id": id, "announcement_type": announcement.Type})
			_, err = model.CreateDingTalkNotification(&model.DingTalkNotification{
				EventType: model.DingTalkNotificationEventAnnouncementGroup,
				DedupeKey: announcementGroupDedupeKey(id), UserId: operatorId, Username: operatorName,
				Title: "系统公告", Content: announcementMarkdownContent(announcement, displayPublishAt),
				Status: model.DingTalkNotificationStatusSkipped, Error: "DingTalk announcement group is not configured",
				ScheduledAt: publishAt.Unix(), Metadata: string(metadata),
			})
			if err != nil {
				return err
			}
			continue
		}
		if conversationID == "" {
			continue
		}
		metadata, err := common.Marshal(map[string]any{"announcement_id": id, "announcement_type": announcement.Type})
		if err != nil {
			return err
		}
		notification := &model.DingTalkNotification{
			EventType: model.DingTalkNotificationEventAnnouncementGroup,
			DedupeKey: announcementGroupDedupeKey(id), UserId: operatorId, Username: operatorName,
			Recipient: conversationID, Title: "系统公告", Content: announcementMarkdownContent(announcement, displayPublishAt),
			ScheduledAt: publishAt.Unix(), Metadata: string(metadata),
		}
		if existed {
			_, err = model.UpdatePendingAnnouncementGroupNotification(notification)
		} else {
			_, err = model.UpsertPendingAnnouncementGroupNotification(notification)
		}
		if err != nil {
			return err
		}
		if publishAt.Unix() <= now.Unix() {
			hasDue = true
		}
	}
	for id := range previousById {
		if _, exists := currentById[id]; !exists {
			if err := model.CancelPendingAnnouncementGroupNotification(announcementGroupDedupeKey(id), "announcement deleted before group delivery"); err != nil {
				return err
			}
		}
	}
	if hasDue {
		_, _, err = EnqueueSystemTask(model.SystemTaskTypeAnnouncementGroupDelivery, struct{}{})
	}
	return err
}

func deliverDueAnnouncementGroupNotifications(ctx context.Context, now int64) (announcementGroupDeliveryResult, error) {
	result := announcementGroupDeliveryResult{}
	for result.Attempted < announcementGroupDeliveryBatchSize {
		records, err := model.ClaimDueAnnouncementGroupNotifications(now, 1)
		if err != nil {
			return result, err
		}
		if len(records) == 0 {
			break
		}
		record := records[0]
		result.Attempted++
		sentAt := time.Now().Unix()
		settings := *system_setting.GetDingTalkSettings()
		conversationID := strings.TrimSpace(settings.AnnouncementGroupOpenConversationId)
		if conversationID == "" {
			result.Failed++
			if err := model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusFailed, ErrDingTalkGroupNotConfigured.Error(), sentAt); err != nil {
				return result, err
			}
			continue
		}
		if conversationID != record.Recipient {
			if err := model.DB.Model(&model.DingTalkNotification{}).Where("id = ?", record.Id).Update("recipient", conversationID).Error; err != nil {
				return result, err
			}
		}
		if err := sendAnnouncementGroupMessage(ctx, settings, conversationID, record.Title, record.Content); err != nil {
			result.Failed++
			if updateErr := model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusFailed, err.Error(), sentAt); updateErr != nil {
				return result, updateErr
			}
			continue
		}
		result.Succeeded++
		if err := model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusSucceeded, "", sentAt); err != nil {
			return result, err
		}
	}
	return result, nil
}

type announcementGroupDeliveryHandler struct{}

func (announcementGroupDeliveryHandler) Type() string {
	return model.SystemTaskTypeAnnouncementGroupDelivery
}
func (announcementGroupDeliveryHandler) Enabled() bool {
	now := common.GetTimestamp()
	if err := model.FailStaleRunningAnnouncementGroupNotifications(now - 5*60); err != nil {
		common.SysError("fail stale announcement group notifications: " + err.Error())
	}
	if strings.TrimSpace(system_setting.GetDingTalkSettings().AnnouncementGroupOpenConversationId) == "" {
		return false
	}
	due, err := model.HasDueAnnouncementGroupNotifications(now)
	if err != nil {
		common.SysError("check due announcement group notifications: " + err.Error())
	}
	return err == nil && due
}
func (announcementGroupDeliveryHandler) Interval() time.Duration { return time.Minute }
func (announcementGroupDeliveryHandler) NewPayload() any         { return struct{}{} }
func (announcementGroupDeliveryHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	result, err := deliverDueAnnouncementGroupNotifications(ctx, common.GetTimestamp())
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logSystemTaskLockError(ctx, task, err)
	}
}

func init() {
	RegisterSystemTaskHandler(announcementGroupDeliveryHandler{})
}
