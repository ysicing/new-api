package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const dingTalkAPIBaseURL = "https://api.dingtalk.com"
const dingTalkLegacyAPIBaseURL = "https://oapi.dingtalk.com"

type DingTalkNotificationRequest struct {
	EventType string
	DedupeKey string
	UserId    int
	UserEmail string
	Title     string
	Content   string
	Metadata  map[string]any
}

type dingTalkNotifier struct {
	httpClient    *http.Client
	baseURL       string
	legacyBaseURL string
	now           func() time.Time

	tokenMutex     sync.Mutex
	accessToken    string
	tokenExpiresAt time.Time
	tokenAppKey    string
	tokenAppSecret string

	legacyAccessToken    string
	legacyTokenExpiresAt time.Time
	legacyTokenAppKey    string
	legacyTokenAppSecret string
}

func newDingTalkNotifier(httpClient *http.Client, baseURL string) *dingTalkNotifier {
	return newDingTalkNotifierWithURLs(httpClient, baseURL, baseURL)
}

func newDingTalkNotifierWithURLs(httpClient *http.Client, baseURL, legacyBaseURL string) *dingTalkNotifier {
	return &dingTalkNotifier{
		httpClient: httpClient, baseURL: strings.TrimRight(baseURL, "/"),
		legacyBaseURL: strings.TrimRight(legacyBaseURL, "/"), now: time.Now,
	}
}

var defaultDingTalkNotifier = newDingTalkNotifierWithURLs(nil, dingTalkAPIBaseURL, dingTalkLegacyAPIBaseURL)

func (notifier *dingTalkNotifier) client() *http.Client {
	if notifier.httpClient != nil {
		return notifier.httpClient
	}
	if client := GetHttpClient(); client != nil {
		return client
	}
	return http.DefaultClient
}

// dingTalkUserIDFromEmail 按企业通讯录约定使用邮箱第一个 @ 之前的部分作为钉钉 userId。
func dingTalkUserIDFromEmail(email string) (string, bool) {
	email = strings.TrimSpace(email)
	userID, domain, found := strings.Cut(email, "@")
	userID = strings.TrimSpace(userID)
	if !found || userID == "" || strings.TrimSpace(domain) == "" {
		return "", false
	}
	return userID, true
}

// DispatchDingTalkNotification 创建可查询的投递记录并同步完成一次机器人发送。
// 调用方可在自己的异步任务中调用，避免外部网络请求阻塞业务响应。
func DispatchDingTalkNotification(ctx context.Context, request DingTalkNotificationRequest) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	record, err := newDingTalkNotificationRecord(request)
	if err != nil {
		return false, err
	}
	created, err := model.CreateDingTalkNotification(record)
	if err != nil || !created {
		return created, err
	}
	return true, deliverDingTalkNotification(ctx, request, record)
}

func dispatchHourlyRateLimitedDingTalkNotification(ctx context.Context, request DingTalkNotificationRequest, occurredAt time.Time, maxCount int) (bool, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	record, err := newDingTalkNotificationRecord(request)
	if err != nil {
		return false, false, err
	}
	windowStart := time.Date(occurredAt.Year(), occurredAt.Month(), occurredAt.Day(), occurredAt.Hour(), 0, 0, 0, occurredAt.Location())
	windowEnd := windowStart.Add(time.Hour)
	record.CreatedAt = occurredAt.Unix()
	created, allowed, err := model.CreateHourlyRateLimitedDingTalkNotification(record, windowStart.Unix(), windowEnd.Unix(), maxCount)
	if err != nil || !created || !allowed {
		return created, allowed, err
	}
	return true, true, deliverDingTalkNotification(ctx, request, record)
}

func newDingTalkNotificationRecord(request DingTalkNotificationRequest) (*model.DingTalkNotification, error) {
	metadata, err := common.Marshal(request.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal DingTalk notification metadata: %w", err)
	}
	recipient, _ := dingTalkUserIDFromEmail(request.UserEmail)
	username, _ := model.GetUsernameById(request.UserId, false)
	return &model.DingTalkNotification{
		EventType: request.EventType, DedupeKey: request.DedupeKey,
		UserId: request.UserId, Username: username, Recipient: recipient,
		Title: request.Title, Content: request.Content,
		Status: model.DingTalkNotificationStatusPending, Metadata: string(metadata),
	}, nil
}

func deliverDingTalkNotification(ctx context.Context, request DingTalkNotificationRequest, record *model.DingTalkNotification) error {
	settings := *system_setting.GetDingTalkSettings()
	if !settings.IsRobotConfigured() {
		errText := "DingTalk robot is not configured"
		return model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusSkipped, errText, 0)
	}
	resolved, err := defaultDingTalkNotifier.resolveRecipient(ctx, settings, DingTalkRecipientRequest{
		UserId: request.UserId, UserEmail: request.UserEmail,
	})
	if err != nil {
		status := model.DingTalkNotificationStatusFailed
		if errors.Is(err, ErrDingTalkInvalidEmail) {
			status = model.DingTalkNotificationStatusSkipped
		}
		return model.UpdateDingTalkNotificationResult(record.Id, status, err.Error(), 0)
	}
	recipient := resolved.StaffUserId
	if err := model.DB.Model(&model.DingTalkNotification{}).Where("id = ?", record.Id).
		Update("recipient", recipient).Error; err != nil {
		return err
	}

	sentAt := time.Now().Unix()
	if err := defaultDingTalkNotifier.Send(ctx, settings, recipient, request.Title, request.Content); err != nil {
		if updateErr := model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusFailed, err.Error(), sentAt); updateErr != nil {
			return fmt.Errorf("send DingTalk notification: %w; update delivery record: %v", err, updateErr)
		}
		return err
	}
	if err := model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusSucceeded, "", sentAt); err != nil {
		return fmt.Errorf("update DingTalk delivery record: %w", err)
	}
	return nil
}

func (notifier *dingTalkNotifier) Send(ctx context.Context, settings system_setting.DingTalkSettings, userID, title, content string) error {
	accessToken, err := notifier.getAccessToken(ctx, settings)
	if err != nil {
		return err
	}
	msgParam, err := common.Marshal(struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	}{Title: title, Text: "### " + title + "\n\n" + content})
	if err != nil {
		return fmt.Errorf("marshal DingTalk message params: %w", err)
	}
	payload, err := common.Marshal(struct {
		RobotCode string   `json:"robotCode"`
		UserIDs   []string `json:"userIds"`
		MsgKey    string   `json:"msgKey"`
		MsgParam  string   `json:"msgParam"`
	}{
		RobotCode: strings.TrimSpace(settings.ClientId), UserIDs: []string{userID},
		MsgKey: "sampleMarkdown", MsgParam: string(msgParam),
	})
	if err != nil {
		return fmt.Errorf("marshal DingTalk message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notifier.baseURL+"/v1.0/robot/oToMessages/batchSend", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create DingTalk message request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", accessToken)
	resp, err := notifier.client().Do(req)
	if err != nil {
		return fmt.Errorf("send DingTalk message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code                      string   `json:"code"`
		Message                   string   `json:"message"`
		ProcessQueryKey           string   `json:"processQueryKey"`
		InvalidStaffIDList        []string `json:"invalidStaffIdList"`
		FlowControlledStaffIDList []string `json:"flowControlledStaffIdList"`
	}
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return fmt.Errorf("decode DingTalk message response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || result.Code != "" {
		return fmt.Errorf("DingTalk message request failed: status=%d code=%s message=%s", resp.StatusCode, result.Code, result.Message)
	}
	if slices.Contains(result.InvalidStaffIDList, userID) {
		return fmt.Errorf("DingTalk message request failed: invalid staff id")
	}
	if slices.Contains(result.FlowControlledStaffIDList, userID) {
		return fmt.Errorf("DingTalk message request failed: recipient is flow controlled")
	}
	if strings.TrimSpace(result.ProcessQueryKey) == "" {
		return fmt.Errorf("DingTalk message request failed: missing process query key")
	}
	return nil
}

func (notifier *dingTalkNotifier) getAccessToken(ctx context.Context, settings system_setting.DingTalkSettings) (string, error) {
	notifier.tokenMutex.Lock()
	defer notifier.tokenMutex.Unlock()

	appKey := strings.TrimSpace(settings.ClientId)
	appSecret := strings.TrimSpace(settings.ClientSecret)
	now := notifier.now()
	if notifier.accessToken != "" && notifier.tokenAppKey == appKey && notifier.tokenAppSecret == appSecret && now.Before(notifier.tokenExpiresAt) {
		return notifier.accessToken, nil
	}
	payload, err := common.Marshal(struct {
		AppKey    string `json:"appKey"`
		AppSecret string `json:"appSecret"`
	}{AppKey: appKey, AppSecret: appSecret})
	if err != nil {
		return "", fmt.Errorf("marshal DingTalk token request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notifier.baseURL+"/v1.0/oauth2/accessToken", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create DingTalk token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := notifier.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("request DingTalk access token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
		Code        string `json:"code"`
		Message     string `json:"message"`
	}
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return "", fmt.Errorf("decode DingTalk token response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || result.Code != "" || result.AccessToken == "" {
		return "", fmt.Errorf("DingTalk token request failed: status=%d code=%s message=%s", resp.StatusCode, result.Code, result.Message)
	}
	cacheDuration := time.Duration(result.ExpireIn) * time.Second
	if cacheDuration > time.Minute {
		cacheDuration -= time.Minute
	}
	notifier.accessToken = result.AccessToken
	notifier.tokenExpiresAt = now.Add(cacheDuration)
	notifier.tokenAppKey = appKey
	notifier.tokenAppSecret = appSecret
	return notifier.accessToken, nil
}
