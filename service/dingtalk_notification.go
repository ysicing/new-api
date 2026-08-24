package service

import (
	"bytes"
	"context"
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
	httpClient *http.Client
	baseURL    string
	now        func() time.Time

	tokenMutex     sync.Mutex
	accessToken    string
	tokenExpiresAt time.Time
	tokenAppKey    string
	tokenAppSecret string
}

func newDingTalkNotifier(httpClient *http.Client, baseURL string) *dingTalkNotifier {
	return &dingTalkNotifier{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
		now:        time.Now,
	}
}

var defaultDingTalkNotifier = newDingTalkNotifier(nil, dingTalkAPIBaseURL)

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
	metadata, err := common.Marshal(request.Metadata)
	if err != nil {
		return false, fmt.Errorf("marshal DingTalk notification metadata: %w", err)
	}
	recipient, hasRecipient := dingTalkUserIDFromEmail(request.UserEmail)
	username, _ := model.GetUsernameById(request.UserId, false)
	record := &model.DingTalkNotification{
		EventType: request.EventType, DedupeKey: request.DedupeKey,
		UserId: request.UserId, Username: username, Recipient: recipient,
		Title: request.Title, Content: request.Content,
		Status: model.DingTalkNotificationStatusPending, Metadata: string(metadata),
	}
	created, err := model.CreateDingTalkNotification(record)
	if err != nil || !created {
		return created, err
	}

	settings := *system_setting.GetDingTalkSettings()
	if !hasRecipient {
		errText := "user email is missing or invalid"
		return true, model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusSkipped, errText, 0)
	}
	if !settings.IsRobotConfigured() {
		errText := "DingTalk robot is not configured"
		return true, model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusSkipped, errText, 0)
	}

	sentAt := time.Now().Unix()
	if err := defaultDingTalkNotifier.Send(ctx, settings, recipient, request.Title, request.Content); err != nil {
		if updateErr := model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusFailed, err.Error(), sentAt); updateErr != nil {
			return true, fmt.Errorf("send DingTalk notification: %w; update delivery record: %v", err, updateErr)
		}
		return true, err
	}
	if err := model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusSucceeded, "", sentAt); err != nil {
		return true, fmt.Errorf("update DingTalk delivery record: %w", err)
	}
	return true, nil
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
		RobotCode: strings.TrimSpace(settings.RobotCode), UserIDs: []string{userID},
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
