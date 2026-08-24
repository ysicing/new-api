package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

var (
	ErrDingTalkNotConfigured = errors.New("DingTalk robot is not configured")
	ErrDingTalkSendFailed    = errors.New("DingTalk robot message send failed")
)

type DingTalkTestMessageResult struct {
	BoundNow bool `json:"bound_now"`
}

func SendDingTalkTestMessage(ctx context.Context, userId int) (DingTalkTestMessageResult, error) {
	settings := *system_setting.GetDingTalkSettings()
	if !settings.IsRobotConfigured() {
		return DingTalkTestMessageResult{}, ErrDingTalkNotConfigured
	}
	user, err := model.GetUserById(userId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		return DingTalkTestMessageResult{}, ErrDingTalkIdentityMismatch
	}
	recipient, _ := dingTalkUserIDFromEmail(user.Email)
	title := common.SystemName + " DingTalk Bot Test"
	content := fmt.Sprintf("Platform user: %s (ID: %d)\n\nSent at: %s\n\nThis message verifies DingTalk personal notification delivery.", user.Username, user.Id, time.Now().Format(time.RFC3339))
	record, err := createDingTalkTestRecord(user, recipient, title, content)
	if err != nil {
		return DingTalkTestMessageResult{}, err
	}
	resolved, err := defaultDingTalkNotifier.resolveRecipient(ctx, settings, DingTalkRecipientRequest{UserId: user.Id, UserEmail: user.Email})
	if err != nil {
		_ = model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusFailed, err.Error(), 0)
		return DingTalkTestMessageResult{}, err
	}
	if err := model.DB.Model(record).Update("recipient", resolved.StaffUserId).Error; err != nil {
		return DingTalkTestMessageResult{}, err
	}
	sentAt := time.Now().Unix()
	if err := defaultDingTalkNotifier.Send(ctx, settings, resolved.StaffUserId, title, content); err != nil {
		_ = model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusFailed, err.Error(), sentAt)
		return DingTalkTestMessageResult{}, fmt.Errorf("%w: %v", ErrDingTalkSendFailed, err)
	}
	if err := model.UpdateDingTalkNotificationResult(record.Id, model.DingTalkNotificationStatusSucceeded, "", sentAt); err != nil {
		return DingTalkTestMessageResult{}, err
	}
	return DingTalkTestMessageResult{BoundNow: resolved.BoundNow}, nil
}

func createDingTalkTestRecord(user *model.User, recipient, title, content string) (*model.DingTalkNotification, error) {
	metadata, err := common.Marshal(map[string]any{"test": true})
	if err != nil {
		return nil, err
	}
	record := &model.DingTalkNotification{
		EventType: model.DingTalkNotificationEventTest,
		DedupeKey: "test:" + common.NewRequestId(), UserId: user.Id,
		Username: user.Username, Recipient: recipient, Title: title, Content: content,
		Status: model.DingTalkNotificationStatusPending, Metadata: string(metadata),
	}
	created, err := model.CreateDingTalkNotification(record)
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, errors.New("DingTalk test notification dedupe collision")
	}
	return record, nil
}
