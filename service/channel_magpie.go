package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
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
		Used      *float64   `json:"used"`
		Unlimited bool       `json:"unlimited"`
		ResetsAt  *time.Time `json:"resetsAt"`
		ResetSecs *int64     `json:"resetSecs"`
	} `json:"windows"`
}

type magpieQuotaResponse struct {
	Object string            `json:"object"`
	Data   []magpieQuotaCard `json:"data"`
}

type magpieCodexAvailability struct {
	Available   bool
	NextCheckAt time.Time
}

func queryMagpieCodexAvailability(ctx context.Context, channel *model.Channel) (magpieCodexAvailability, error) {
	var settings dto.ChannelSettings
	if channel.Setting != nil && *channel.Setting != "" {
		if err := common.UnmarshalJsonStr(*channel.Setting, &settings); err != nil {
			return magpieCodexAvailability{}, errors.New("invalid channel transport settings")
		}
	}
	var other dto.ChannelOtherSettings
	if err := common.UnmarshalJsonStr(channel.OtherSettings, &other); err != nil {
		return magpieCodexAvailability{}, errors.New("invalid Magpie channel settings")
	}
	client, err := GetHttpClientWithProxySettings(settings.Proxy, settings)
	if err != nil {
		return magpieCodexAvailability{}, errors.New("invalid channel proxy settings")
	}
	// 保留渠道的代理/连接策略，拒绝重定向，防止网关密钥被转发到其他地址。
	queryClient := *client
	queryClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	base, err := url.Parse(strings.TrimSpace(channel.GetBaseURL()))
	if err != nil || base == nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return magpieCodexAvailability{}, errors.New("invalid Magpie channel URL")
	}
	base.Path = strings.TrimSuffix(strings.TrimRight(base.Path, "/"), "/v1") + "/v1/magpie/quotas"
	base.RawPath = ""
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	keys := channel.GetKeys()
	if len(keys) == 0 {
		return magpieCodexAvailability{}, errors.New("Magpie gateway key is empty")
	}
	unknownReasons := make(map[string]int)
	readingTimes := make(map[string]struct{ oldest, newest time.Time })
	var checkedAt time.Time
	var lastRequestErr error
	queried := false
	var nextCheckAt time.Time
	resetsKnown := true
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
			unknownReasons["GET 额度查询失败"]++
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
			unknownReasons["POST 额度刷新失败"]++
			lastRequestErr = err
			continue
		}
		checkedAt = time.Now().UTC()
		for _, card := range cards.Data {
			if card.Provider != "codex" || card.Kind != "subscription" {
				continue
			}
			delete(expectedAccounts, card.User)
			// 只记录校验失败类型，不直接写入上游 error 或账号信息，避免
			// 将上游可能包含的凭证、地址或邮箱带入系统日志和任务详情。
			reason := ""
			invalidReadingTime := false
			switch {
			case card.Error != "":
				reason = "Codex 账号额度查询返回错误（error 非空）"
			case card.AsOf != nil:
				reason = "Codex 账号额度使用历史回退数据（asOf 非空）"
			case card.ReadAt == nil:
				reason = "Codex 账号额度缺少读取时间（readAt）"
			// 容忍 Magpie 短期缓存及小幅时钟偏差；超过 10 分钟的数据
			// 仍视为未知，避免凭长期旧卡片切换渠道状态。
			case checkedAt.Sub(*card.ReadAt) > 10*time.Minute:
				reason = "Codex 账号额度读取时间已超过 10 分钟"
				invalidReadingTime = true
			case card.ReadAt.Sub(checkedAt) > time.Minute:
				reason = "Codex 账号额度读取时间超前超过 1 分钟，请检查时钟"
				invalidReadingTime = true
			case len(card.Windows) == 0:
				reason = "Codex 账号额度缺少用量窗口（windows）"
			}
			if reason != "" {
				unknownReasons[reason]++
				if invalidReadingTime {
					// 按失败原因聚合时间范围，避免账号较多时日志膨胀；只保留
					// 时间证据，不输出账号标识、密钥或完整上游响应。
					bounds, recorded := readingTimes[reason]
					if !recorded || card.ReadAt.Before(bounds.oldest) {
						bounds.oldest = card.ReadAt.UTC()
					}
					if !recorded || card.ReadAt.After(bounds.newest) {
						bounds.newest = card.ReadAt.UTC()
					}
					readingTimes[reason] = bounds
				}
				continue
			}
			valid := true
			for _, window := range card.Windows {
				if !window.Unlimited && (window.Used == nil || math.IsNaN(*window.Used) || math.IsInf(*window.Used, 0) || *window.Used < 0) {
					valid = false
				}
			}
			if !valid {
				unknownReasons["Codex 账号用量窗口缺少有效 used 值"]++
				continue
			}
			if !card.Held {
				return magpieCodexAvailability{Available: true}, nil
			}
			accountNext := magpieAccountNextCheckAt(card, time.Now())
			if accountNext.IsZero() {
				resetsKnown = false
			} else if nextCheckAt.IsZero() || accountNext.Before(nextCheckAt) {
				nextCheckAt = accountNext
			}
		}
		if len(expectedAccounts) > 0 {
			unknownReasons["刷新响应缺少原有 Codex 账号"] += len(expectedAccounts)
		}
	}
	// 有一个确定可用的账号即可启用；只有所有账号均确定不可用时才禁用。
	if !queried {
		unknownReasons["没有已启用的网关密钥"]++
	}
	if len(unknownReasons) > 0 {
		reasons := make([]string, 0, len(unknownReasons))
		for reason, count := range unknownReasons {
			detail := fmt.Sprintf("%s（%d）", reason, count)
			if bounds, ok := readingTimes[reason]; ok {
				detail += fmt.Sprintf(" [检查时间（UTC）：%s; readAt 范围（UTC）：%s ~ %s; 数据年龄范围：%s ~ %s（允许 -1m0s 至 10m0s）]", checkedAt.Format(time.RFC3339Nano), bounds.oldest.Format(time.RFC3339Nano), bounds.newest.Format(time.RFC3339Nano), checkedAt.Sub(bounds.newest).Round(time.Millisecond), checkedAt.Sub(bounds.oldest).Round(time.Millisecond))
			}
			reasons = append(reasons, detail)
		}
		sort.Strings(reasons)
		details := strings.Join(reasons, "; ")
		if lastRequestErr != nil {
			return magpieCodexAvailability{}, fmt.Errorf("Magpie Codex account availability is unknown; channel status preserved: %s: %w", details, lastRequestErr)
		}
		return magpieCodexAvailability{}, fmt.Errorf("Magpie Codex account availability is unknown; channel status preserved: %s", details)
	}
	if !resetsKnown {
		nextCheckAt = time.Time{}
	}
	return magpieCodexAvailability{NextCheckAt: nextCheckAt}, nil
}

// magpieAccountNextCheckAt 返回一个暂停账号所有耗尽窗口都重置的时间。
// held 还可能来自其他限制：没有明确耗尽窗口或任一重置时间未知时，不能推迟探测。
func magpieAccountNextCheckAt(card magpieQuotaCard, now time.Time) time.Time {
	var next time.Time
	for _, window := range card.Windows {
		if window.Unlimited || window.Used == nil || *window.Used < 100 {
			continue
		}
		var reset time.Time
		if window.ResetsAt != nil {
			reset = *window.ResetsAt
		} else if card.ReadAt != nil && window.ResetSecs != nil && *window.ResetSecs > 0 && *window.ResetSecs <= math.MaxInt64/int64(time.Second) {
			// 相对秒数属于这次读取，不能每次以当前时间重新延长等待。
			reset = card.ReadAt.Add(time.Duration(*window.ResetSecs) * time.Second)
		}
		if !reset.After(now) {
			return time.Time{}
		}
		if reset.After(next) {
			next = reset
		}
	}
	return next
}

// 缓存按渠道配置及本次自动禁用标记隔离；轮询游标不参与指纹，
// 密钥和代理凭证只用于摘要计算，不出现在 Redis key/value 或任务日志中。
func magpieProbeCacheKey(channel *model.Channel) string {
	config, err := common.Marshal(struct {
		Type               int
		BaseURL, Setting   *string
		Key, OtherSettings string
		MultiKey           bool
		KeyStatuses        map[int]int
		DisabledAt         int64
	}{channel.Type, channel.BaseURL, channel.Setting, channel.Key, channel.OtherSettings, channel.ChannelInfo.IsMultiKey, channel.ChannelInfo.MultiKeyStatusList, channel.MagpieAutoDisabledAt})
	if err != nil {
		return ""
	}
	return fmt.Sprintf("magpie_channel_probe:v1:%d:%x", channel.Id, sha256.Sum256(config))
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

func notifyMagpieChannelStatus(ctx context.Context, channel *model.Channel, transitionAt int64, recovered bool, nextCheckAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	event, title := "magpie_channel_disabled", "Magpie 自建渠道已自动禁用"
	content := ""
	if recovered {
		event, title = "magpie_channel_enabled", "Magpie 自建渠道已自动启用"
		content = "原因：检测到可用账号，渠道已自动启用，可继续使用。"
	} else {
		earliest := "暂无法确定"
		if nextCheckAt.After(time.Now()) {
			// 使用账号窗口的恢复时间，不能用最长 30 分钟的缓存探测时间替代。
			earliest = nextCheckAt.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02 15:04:05") + "（北京时间）"
		}
		content = "原因：无可用账号，渠道已自动禁用，等待小时或周限制恢复后启用。\n\n可能最早启用时间：" + earliest + "\n实际启用以探测到可用账号为准。"
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
	Skipped            int                         `json:"skipped"`
	Changed            int                         `json:"changed"`
	Enabled            int                         `json:"enabled"`
	Disabled           int                         `json:"disabled"`
	Unchanged          int                         `json:"unchanged"`
	Failed             int                         `json:"failed"`
	NotificationFailed int                         `json:"notification_failed"`
	Channels           []magpieChannelMonitorEntry `json:"channels"`
}

type magpieChannelMonitorEntry struct {
	ChannelID         int        `json:"channel_id"`
	ChannelName       string     `json:"channel_name"`
	Available         *bool      `json:"available,omitempty"`
	Action            string     `json:"action"`
	Error             string     `json:"error,omitempty"`
	NotificationError string     `json:"notification_error,omitempty"`
	NextCheckAt       *time.Time `json:"next_check_at,omitempty"`
}

func syncMagpieChannels(ctx context.Context, manual bool) (magpieChannelMonitorResult, error) {
	result := magpieChannelMonitorResult{Channels: make([]magpieChannelMonitorEntry, 0)}
	channels, err := model.ListMagpieChannels(ctx)
	if err != nil {
		return result, err
	}
	type notice struct {
		channel      *model.Channel
		transitionAt int64
		resultIndex  int
		nextCheckAt  time.Time
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
		// Redis 仅提供减少探测的提示。失败、未启用或配置变化时回退到实际查询，
		// 缓存本身不能启用渠道，也不能覆盖人工状态操作。
		if !manual && common.RedisEnabled && common.RDB != nil && channel.Status == common.ChannelStatusAutoDisabled && channel.MagpieAutoDisabledAt != 0 {
			cacheCtx, cancel := context.WithTimeout(ctx, time.Second)
			cached, cacheErr := common.RDB.Get(cacheCtx, magpieProbeCacheKey(channel)).Result()
			cancel()
			next, parseErr := time.Parse(time.RFC3339Nano, cached)
			now := time.Now()
			if cacheErr == nil && parseErr == nil && next.After(now) && !next.After(now.Add(magpieChannelProbeCacheTTL(now))) {
				entry.Action, entry.NextCheckAt = "cached", &next
				result.Skipped++
				continue
			}
		}
		result.Checked++
		availability, queryErr := queryMagpieCodexAvailability(ctx, channel)
		if queryErr != nil {
			entry.Action, entry.Error = "query_failed", queryErr.Error()
			result.Failed++
			common.SysError(fmt.Sprintf("query Magpie channel #%d: %v", channel.Id, queryErr))
			continue
		}
		available := availability.Available
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
			notices = append(notices, notice{channel: changed, transitionAt: transitionAt, resultIndex: resultIndex, nextCheckAt: availability.NextCheckAt})
		}
		if changed != nil {
			channel = changed
		}
		if !available && channel.Status == common.ChannelStatusAutoDisabled && channel.MagpieAutoDisabledAt != 0 && common.RedisEnabled && common.RDB != nil {
			now := time.Now()
			next := availability.NextCheckAt
			if limit := now.Add(magpieChannelProbeCacheTTL(now)); next.After(limit) {
				next = limit
			}
			if next.After(now) {
				cacheCtx, cancel := context.WithTimeout(ctx, time.Second)
				cacheErr := common.RDB.Set(cacheCtx, magpieProbeCacheKey(channel), next.Format(time.RFC3339Nano), next.Sub(now)).Err()
				cancel()
				if cacheErr == nil {
					entry.NextCheckAt = &next
				}
			}
		}
	}
	// 先完成状态切换，再发送通知；外部通知失败不会回滚渠道状态。
	notificationCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for _, notice := range notices {
		if err := notifyMagpieChannelStatus(notificationCtx, notice.channel, notice.transitionAt, notice.channel.Status == common.ChannelStatusEnabled, notice.nextCheckAt); err != nil {
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

// MagpieChannelMonitorPayload 区分人工检查和定时检查；旧任务缺少 manual 时仍按定时检查处理。
type MagpieChannelMonitorPayload struct {
	Manual bool `json:"manual,omitempty"`
}

// 白天缩短缓存以发现官方提前重置、补充额度或新增账号；夜间维持低频探测。
func magpieChannelProbeCacheTTL(now time.Time) time.Duration {
	hour := now.UTC().Add(8 * time.Hour).Hour()
	if hour >= 22 || hour < 9 {
		return 30 * time.Minute
	}
	return 5 * time.Minute
}

func (magpieChannelMonitorHandler) NewPayload() any { return MagpieChannelMonitorPayload{} }
func (magpieChannelMonitorHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	payload := MagpieChannelMonitorPayload{}
	err := task.DecodePayload(&payload)
	result := magpieChannelMonitorResult{}
	if err == nil {
		result, err = syncMagpieChannels(ctx, payload.Manual)
	}
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
