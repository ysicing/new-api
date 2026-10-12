package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// 使用 Magpie 自己的 held 判断：额度百分比为 100 时，credits 仍可能允许使用。
// 不能仅凭五小时/周窗口的 used 值或预计重置时间启用渠道。
type magpieQuotaCard struct {
	Provider string     `json:"provider"`
	User     string     `json:"user"`
	Kind     string     `json:"kind"`
	Error    string     `json:"error"`
	Held     bool       `json:"held"`
	AsOf     *time.Time `json:"asOf"`
	ReadAt   *time.Time `json:"readAt"`
	Windows  []struct {
		Used      *float64 `json:"used"`
		Unlimited bool     `json:"unlimited"`
	} `json:"windows"`
}

type magpieQuotaResponse struct {
	Object string            `json:"object"`
	Data   []magpieQuotaCard `json:"data"`
}

func queryMagpieCodexAvailability(ctx context.Context, channel *model.Channel) (bool, error) {
	var settings dto.ChannelSettings
	if channel.Setting != nil && *channel.Setting != "" {
		if err := common.UnmarshalJsonStr(*channel.Setting, &settings); err != nil {
			return false, errors.New("invalid channel transport settings")
		}
	}
	var other dto.ChannelOtherSettings
	if err := common.UnmarshalJsonStr(channel.OtherSettings, &other); err != nil {
		return false, errors.New("invalid Magpie channel settings")
	}
	client, err := GetHttpClientWithProxySettings(settings.Proxy, settings)
	if err != nil {
		return false, errors.New("invalid channel proxy settings")
	}
	// 保留渠道的代理/连接策略，拒绝重定向，防止网关密钥被转发到其他地址。
	queryClient := *client
	queryClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	base, err := url.Parse(strings.TrimSpace(channel.GetBaseURL()))
	if err != nil || base == nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return false, errors.New("invalid Magpie channel URL")
	}
	base.Path = strings.TrimSuffix(strings.TrimRight(base.Path, "/"), "/v1") + "/v1/magpie/quotas"
	base.RawPath = ""
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	keys := channel.GetKeys()
	if len(keys) == 0 {
		return false, errors.New("Magpie gateway key is empty")
	}
	unknown := false
	var lastRequestErr error
	queried := false
	for index, key := range keys {
		if channel.ChannelInfo.IsMultiKey {
			if status, ok := channel.ChannelInfo.MultiKeyStatusList[index]; ok && status != common.ChannelStatusEnabled {
				continue
			}
		}
		queried = true
		// GET 初始化 Magpie 的额度缓存；按 provider 刷新前若没有缓存，Magpie 可能仍返回空卡片。
		quotas, err := requestMagpieQuotas(ctx, &queryClient, http.MethodGet, base.String(), key)
		if err != nil {
			unknown = true
			lastRequestErr = err
			continue
		}
		expectedAccounts := make(map[string]bool)
		for _, card := range quotas.Data {
			if card.Provider == "codex" && card.Kind == "subscription" {
				expectedAccounts[card.User] = true
			}
		}
		refresh := *base
		refresh.Path += "/refresh"
		refresh.RawQuery = "provider=codex"
		if len(expectedAccounts) == 0 {
			// 初次 GET 等待额度超过 12 秒时也会返回空列表；局部刷新无法
			// 初始化 Magpie 的 nil 缓存，需完整刷新后才能确认没有 Codex 账号。
			refresh.RawQuery = ""
		}
		cards, err := requestMagpieQuotas(ctx, &queryClient, http.MethodPost, refresh.String(), key)
		if err != nil {
			unknown = true
			lastRequestErr = err
			continue
		}
		for _, card := range cards.Data {
			if card.Provider != "codex" || card.Kind != "subscription" {
				continue
			}
			delete(expectedAccounts, card.User)
			if card.Error != "" || card.AsOf != nil || card.ReadAt == nil || time.Since(*card.ReadAt) > 3*time.Minute || time.Until(*card.ReadAt) > time.Minute || len(card.Windows) == 0 {
				unknown = true
				continue
			}
			valid := true
			for _, window := range card.Windows {
				if !window.Unlimited && (window.Used == nil || math.IsNaN(*window.Used) || math.IsInf(*window.Used, 0) || *window.Used < 0) {
					valid = false
				}
			}
			if !valid {
				unknown = true
				continue
			}
			if !card.Held {
				return true, nil
			}
		}
		if len(expectedAccounts) > 0 {
			unknown = true
		}
	}
	// 有一个确定可用的账号即可启用；只有所有账号均确定不可用时才禁用。
	if unknown || !queried {
		if lastRequestErr != nil {
			return false, fmt.Errorf("Magpie Codex account availability is unknown; channel status preserved: %w", lastRequestErr)
		}
		return false, errors.New("Magpie Codex account availability is unknown; channel status preserved")
	}
	return false, nil
}

func requestMagpieQuotas(ctx context.Context, client *http.Client, method, endpoint, key string) (*magpieQuotaResponse, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("Magpie gateway key is empty")
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, errors.New("invalid Magpie quota request")
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(key))
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("Magpie quota request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Magpie quota request returned HTTP %d", resp.StatusCode)
	}
	const maxBody = 2 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		return nil, errors.New("invalid Magpie quota response size")
	}
	var result magpieQuotaResponse
	if err := common.Unmarshal(body, &result); err != nil || result.Object != "list" || result.Data == nil {
		return nil, errors.New("invalid Magpie quota response")
	}
	return &result, nil
}

var sendMagpieChannelGroupMessage = func(ctx context.Context, settings system_setting.DingTalkSettings, recipient, title, content string) error {
	return defaultDingTalkNotifier.SendGroup(ctx, settings, recipient, title, content)
}

func notifyMagpieChannelStatus(ctx context.Context, channel *model.Channel, transitionAt int64, recovered bool) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	event, title := "magpie_channel_disabled", "Magpie 渠道已自动禁用"
	content := fmt.Sprintf("渠道：%s（#%d）\n\n原因：Magpie 无可用 Codex 账号，已自动禁用；继续查询账号状态后恢复。", channel.Name, channel.Id)
	if recovered {
		event, title = "magpie_channel_enabled", "Magpie 渠道已自动启用"
		content = fmt.Sprintf("渠道：%s（#%d）\n\nMagpie 已有可用 Codex 账号，已重新启用。", channel.Name, channel.Id)
	}
	settings := *system_setting.GetDingTalkSettings()
	recipient := strings.TrimSpace(settings.AnnouncementGroupOpenConversationId)
	metadata, err := common.Marshal(map[string]any{"channel_id": channel.Id, "transition_at": transitionAt})
	if err != nil {
		return err
	}
	record := &model.DingTalkNotification{
		EventType: event, DedupeKey: fmt.Sprintf("%s:%d:%d", event, channel.Id, transitionAt),
		Recipient: recipient, Title: title, Content: content, Metadata: string(metadata),
		Status: model.DingTalkNotificationStatusRunning,
	}
	created, err := model.CreateDingTalkNotification(record)
	if err != nil || !created {
		return err
	}
	// 通知失败独立记录，不能回滚渠道状态或妨碍后续监控。
	if recipient == "" {
		err = ErrDingTalkGroupNotConfigured
	} else {
		err = sendMagpieChannelGroupMessage(ctx, settings, recipient, title, content)
	}
	status, errorText := model.DingTalkNotificationStatusSucceeded, ""
	if err != nil {
		status, errorText = model.DingTalkNotificationStatusFailed, err.Error()
	}
	if updateErr := model.UpdateDingTalkNotificationResult(record.Id, status, errorText, common.GetTimestamp()); updateErr != nil {
		return updateErr
	}
	return err
}

type magpieChannelMonitorResult struct {
	Checked            int                         `json:"checked"`
	Changed            int                         `json:"changed"`
	Enabled            int                         `json:"enabled"`
	Disabled           int                         `json:"disabled"`
	Unchanged          int                         `json:"unchanged"`
	Failed             int                         `json:"failed"`
	NotificationFailed int                         `json:"notification_failed"`
	Channels           []magpieChannelMonitorEntry `json:"channels"`
}

type magpieChannelMonitorEntry struct {
	ChannelID         int    `json:"channel_id"`
	ChannelName       string `json:"channel_name"`
	Available         *bool  `json:"available,omitempty"`
	Action            string `json:"action"`
	Error             string `json:"error,omitempty"`
	NotificationError string `json:"notification_error,omitempty"`
}

func syncMagpieChannels(ctx context.Context) (magpieChannelMonitorResult, error) {
	result := magpieChannelMonitorResult{Channels: make([]magpieChannelMonitorEntry, 0)}
	channels, err := model.ListMagpieChannels(ctx)
	if err != nil {
		return result, err
	}
	type notice struct {
		channel      *model.Channel
		transitionAt int64
		resultIndex  int
	}
	notices := make([]notice, 0)
	for index := range channels {
		if ctx.Err() != nil {
			err = ctx.Err()
			break
		}
		channel := &channels[index]
		resultIndex := len(result.Channels)
		result.Channels = append(result.Channels, magpieChannelMonitorEntry{ChannelID: channel.Id, ChannelName: channel.Name})
		entry := &result.Channels[resultIndex]
		result.Checked++
		available, queryErr := queryMagpieCodexAvailability(ctx, channel)
		if queryErr != nil {
			entry.Action, entry.Error = "query_failed", queryErr.Error()
			result.Failed++
			common.SysError(fmt.Sprintf("query Magpie channel #%d: %v", channel.Id, queryErr))
			continue
		}
		entry.Available = &available
		changed, updateErr := model.SyncMagpieChannelStatus(ctx, channel, available)
		if updateErr != nil {
			entry.Action, entry.Error = "update_failed", updateErr.Error()
			result.Failed++
			err = updateErr
			break
		}
		if changed == nil {
			entry.Action = "unchanged"
			result.Unchanged++
		} else {
			result.Changed++
			if available {
				entry.Action = "enabled"
				result.Enabled++
			} else {
				entry.Action = "disabled"
				result.Disabled++
			}
			transitionAt := changed.MagpieAutoDisabledAt
			if available {
				transitionAt = channel.MagpieAutoDisabledAt
			}
			reason := "Magpie 无可用 Codex 账号，自动禁用"
			if available {
				reason = "Magpie 已有可用 Codex 账号，自动启用"
			}
			// 仅记录已提交的状态切换，独立于后续钉钉发送结果；系统任务无操作用户。
			model.RecordLogWithAdminInfo(0, model.LogTypeSystem, fmt.Sprintf("渠道「%s」（#%d）：%s", changed.Name, changed.Id, reason), map[string]interface{}{
				"channel_id": changed.Id, "channel_name": changed.Name,
				"status": changed.Status, "reason": reason, "transition_at": transitionAt,
			})
			notices = append(notices, notice{changed, transitionAt, resultIndex})
		}
	}
	// 先完成状态切换，再发送通知；外部通知失败不会回滚渠道状态。
	notificationCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for _, notice := range notices {
		if err := notifyMagpieChannelStatus(notificationCtx, notice.channel, notice.transitionAt, notice.channel.Status == common.ChannelStatusEnabled); err != nil {
			result.NotificationFailed++
			result.Channels[notice.resultIndex].NotificationError = err.Error()
			common.SysError(fmt.Sprintf("notify Magpie channel #%d: %v", notice.channel.Id, err))
		}
	}
	return result, err
}

type magpieChannelMonitorHandler struct{}

func (magpieChannelMonitorHandler) Type() string { return model.SystemTaskTypeMagpieChannelMonitor }
func (magpieChannelMonitorHandler) Enabled() bool {
	channels, err := model.ListMagpieChannels(context.Background())
	if err != nil {
		common.SysError("query Magpie channels: " + err.Error())
	}
	return err == nil && len(channels) > 0
}
func (magpieChannelMonitorHandler) Interval() time.Duration {
	return magpieChannelMonitorInterval(time.Now())
}

// 固定按北京时间选择间隔，不受部署机器时区影响；调度器每轮重新判断，
// 09:00 起恢复白天频率，22:00 起降低夜间查询频率。
func magpieChannelMonitorInterval(now time.Time) time.Duration {
	hour := now.UTC().Add(8 * time.Hour).Hour()
	if hour >= 22 || hour < 9 {
		return 30 * time.Minute
	}
	return 2 * time.Minute
}

func (magpieChannelMonitorHandler) NewPayload() any { return struct{}{} }
func (magpieChannelMonitorHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	result, err := syncMagpieChannels(ctx)
	status, errorText := model.SystemTaskStatusSucceeded, ""
	if err != nil {
		status, errorText = model.SystemTaskStatusFailed, err.Error()
		common.SysError(fmt.Sprintf("system task %s failed: %v", task.TaskID, err))
	}
	// 失败任务也保存已经完成的查询和切换，方便排查部分执行结果。
	if err := model.FinishSystemTask(task.TaskID, runnerID, status, result, errorText); err != nil {
		logSystemTaskLockError(ctx, task, err)
	}
}
func init() { RegisterSystemTaskHandler(magpieChannelMonitorHandler{}) }
