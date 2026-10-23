package model

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

// withChannelStatusTransaction 串行更新渠道、监控标记和选路能力，提交成功后才刷新缓存。
// 行锁和条件更新保护多实例竞争；进程内锁与多 Key 轮询使用相同顺序。
func withChannelStatusTransaction(ctx context.Context, id int, change func(*Channel) bool) (*Channel, error) {
	channelStatusLock.Lock()
	defer channelStatusLock.Unlock()
	pollingLock := GetChannelPollingLock(id)
	pollingLock.Lock()
	defer pollingLock.Unlock()
	var changed *Channel
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var channel Channel
		if err := lockForUpdate(tx).First(&channel, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		previousStatus, previousDisabledAt := channel.Status, channel.MagpieAutoDisabledAt
		if !change(&channel) {
			return nil
		}
		updates := map[string]any{
			"status": channel.Status, "other_info": channel.OtherInfo,
			"magpie_auto_disabled_at": channel.MagpieAutoDisabledAt,
		}
		if channel.ChannelInfo.IsMultiKey {
			updates["channel_info"] = channel.ChannelInfo
		}
		result := tx.Model(&Channel{}).Where("id = ? AND status = ? AND magpie_auto_disabled_at = ?", id, previousStatus, previousDisabledAt).Updates(updates)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		if previousStatus != channel.Status {
			if err := tx.Model(&Ability{}).Where("channel_id = ?", id).Update("enabled", channel.Status == common.ChannelStatusEnabled).Error; err != nil {
				return err
			}
		}
		changed = &channel
		return nil
	})
	if err != nil || changed == nil {
		return nil, err
	}
	if common.MemoryCacheEnabled && changed.ChannelInfo.IsMultiKey && changed.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
		// 内存模式的轮询游标未实时落库。此时仍持有轮询锁，保留最新游标，
		// 避免状态切换用数据库中的旧游标替换正在使用的缓存。
		if info, err := CacheGetChannelInfo(id); err == nil && info.IsMultiKey && info.MultiKeyMode == constant.MultiKeyModePolling {
			changed.ChannelInfo.MultiKeyPollingIndex = info.MultiKeyPollingIndex
		}
	}
	CacheUpdateChannel(changed)
	CacheUpdateChannelStatus(id, changed.Status)
	if changed.Status == common.ChannelStatusEnabled {
		// 单独修改缓存状态不会将此前移除的渠道重新加入选路列表。
		InitChannelCache()
	}
	return changed, nil
}

// SyncMagpieChannelStatus 仅应用仍匹配本次查询快照的结果，避免覆盖查询期间的人工操作。
func SyncMagpieChannelStatus(ctx context.Context, snapshot *Channel, available bool) (*Channel, error) {
	return withChannelStatusTransaction(ctx, snapshot.Id, func(channel *Channel) bool {
		if !channel.IsMagpieEnabled() || channel.Status != snapshot.Status || channel.MagpieAutoDisabledAt != snapshot.MagpieAutoDisabledAt || channel.Key != snapshot.Key || channel.Type != snapshot.Type || !reflect.DeepEqual(channel.BaseURL, snapshot.BaseURL) || !reflect.DeepEqual(channel.Setting, snapshot.Setting) || channel.OtherSettings != snapshot.OtherSettings || channel.ChannelInfo.IsMultiKey != snapshot.ChannelInfo.IsMultiKey || !reflect.DeepEqual(channel.ChannelInfo.MultiKeyStatusList, snapshot.ChannelInfo.MultiKeyStatusList) {
			return false
		}
		reason := "Magpie 无可用 Codex 账号，自动禁用"
		if available {
			// 只恢复本监控禁用的渠道，不复活人工或其他原因禁用的渠道/Key。
			if channel.Status != common.ChannelStatusAutoDisabled || channel.MagpieAutoDisabledAt == 0 {
				return false
			}
			channel.Status = common.ChannelStatusEnabled
			channel.MagpieAutoDisabledAt = 0
			reason = "Magpie 已有可用 Codex 账号，自动启用"
		} else {
			if channel.Status != common.ChannelStatusEnabled {
				return false
			}
			channel.Status = common.ChannelStatusAutoDisabled
			channel.MagpieAutoDisabledAt = time.Now().UnixNano()
		}
		info := channel.GetOtherInfo()
		info["status_reason"] = reason
		info["status_time"] = common.GetTimestamp()
		channel.SetOtherInfo(info)
		return true
	})
}

// ListMagpieChannels 在 Go 中解析 settings，避免依赖不同数据库的 JSON 查询语法。
func ListMagpieChannels(ctx context.Context) ([]Channel, error) {
	var candidates []Channel
	if err := DB.WithContext(ctx).Where("status = ? OR (status = ? AND magpie_auto_disabled_at > 0)", common.ChannelStatusEnabled, common.ChannelStatusAutoDisabled).Order("id ASC").Find(&candidates).Error; err != nil {
		return nil, err
	}
	channels := make([]Channel, 0)
	for _, channel := range candidates {
		if channel.IsMagpieEnabled() {
			channels = append(channels, channel)
		}
	}
	return channels, nil
}

// RecordMagpieChannelTransition 仅在监控已提交状态切换后记录系统日志。
func RecordMagpieChannelTransition(channel *Channel, reason string, transitionAt int64) {
	other := NewLogOther()
	other.MergeAdmin(map[string]any{"channel_id": channel.Id, "channel_name": channel.Name, "status": channel.Status, "reason": reason, "transition_at": transitionAt})
	if err := createLog(&Log{CreatedAt: common.GetTimestamp(), Type: LogTypeSystem, Content: fmt.Sprintf("渠道「%s」（#%d）：%s", channel.Name, channel.Id, reason), Other: other.JSONString()}); err != nil {
		common.SysError(err.Error())
	}
}
