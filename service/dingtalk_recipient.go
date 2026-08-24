package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

var (
	ErrDingTalkInvalidEmail     = errors.New("DingTalk user email is invalid")
	ErrDingTalkIdentityMismatch = errors.New("DingTalk identity does not match platform user")
	ErrDingTalkDirectoryFailed  = errors.New("DingTalk directory query failed")
)

type DingTalkRecipientRequest struct {
	UserId    int
	UserEmail string
}

type DingTalkRecipient struct {
	StaffUserId string
	BoundNow    bool
}

type dingTalkDirectoryUser struct {
	UserId  string `json:"userid"`
	UnionId string `json:"unionid"`
	Email   string `json:"email"`
}

func (notifier *dingTalkNotifier) resolveRecipient(ctx context.Context, settings system_setting.DingTalkSettings, request DingTalkRecipientRequest) (DingTalkRecipient, error) {
	user, err := model.GetUserById(request.UserId, false)
	if err != nil {
		if _, ok := dingTalkUserIDFromEmail(request.UserEmail); !ok {
			return DingTalkRecipient{}, ErrDingTalkInvalidEmail
		}
		return DingTalkRecipient{}, ErrDingTalkIdentityMismatch
	}
	if user.Status != common.UserStatusEnabled {
		return DingTalkRecipient{}, ErrDingTalkIdentityMismatch
	}
	email := model.NormalizeEmail(user.Email)
	if email == "" {
		email = model.NormalizeEmail(request.UserEmail)
	}
	staffUserId, ok := dingTalkUserIDFromEmail(email)
	if !ok {
		return DingTalkRecipient{}, ErrDingTalkInvalidEmail
	}
	if strings.TrimSpace(user.DingTalkId) != "" {
		return DingTalkRecipient{StaffUserId: staffUserId}, nil
	}
	directoryUser, err := notifier.getDirectoryUser(ctx, settings, staffUserId)
	if err != nil {
		return DingTalkRecipient{}, err
	}
	if directoryUser.UserId != staffUserId || model.NormalizeEmail(directoryUser.Email) != email || strings.TrimSpace(directoryUser.UnionId) == "" {
		return DingTalkRecipient{}, ErrDingTalkIdentityMismatch
	}
	boundNow, err := model.BindDingTalkIdentity(user.Id, directoryUser.UnionId)
	if err != nil {
		return DingTalkRecipient{}, err
	}
	return DingTalkRecipient{StaffUserId: staffUserId, BoundNow: boundNow}, nil
}

func (notifier *dingTalkNotifier) getDirectoryUser(ctx context.Context, settings system_setting.DingTalkSettings, staffUserId string) (*dingTalkDirectoryUser, error) {
	accessToken, err := notifier.getLegacyAccessToken(ctx, settings)
	if err != nil {
		return nil, err
	}
	payload, err := common.Marshal(struct {
		UserId   string `json:"userid"`
		Language string `json:"language"`
	}{UserId: staffUserId, Language: "zh_CN"})
	if err != nil {
		return nil, ErrDingTalkDirectoryFailed
	}
	endpoint := notifier.legacyBaseURL + "/topapi/v2/user/get?access_token=" + url.QueryEscape(accessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, ErrDingTalkDirectoryFailed
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := notifier.client().Do(req)
	if err != nil {
		return nil, ErrDingTalkDirectoryFailed
	}
	defer resp.Body.Close()
	var result struct {
		ErrCode int                   `json:"errcode"`
		ErrMsg  string                `json:"errmsg"`
		Result  dingTalkDirectoryUser `json:"result"`
	}
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, ErrDingTalkDirectoryFailed
	}
	if resp.StatusCode != http.StatusOK || result.ErrCode != 0 {
		return nil, fmt.Errorf("%w: status=%d code=%d", ErrDingTalkDirectoryFailed, resp.StatusCode, result.ErrCode)
	}
	return &result.Result, nil
}

func (notifier *dingTalkNotifier) getLegacyAccessToken(ctx context.Context, settings system_setting.DingTalkSettings) (string, error) {
	notifier.tokenMutex.Lock()
	defer notifier.tokenMutex.Unlock()
	appKey, appSecret := strings.TrimSpace(settings.ClientId), strings.TrimSpace(settings.ClientSecret)
	now := notifier.now()
	if notifier.legacyAccessToken != "" && notifier.legacyTokenAppKey == appKey && notifier.legacyTokenAppSecret == appSecret && now.Before(notifier.legacyTokenExpiresAt) {
		return notifier.legacyAccessToken, nil
	}
	query := url.Values{"appkey": {appKey}, "appsecret": {appSecret}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, notifier.legacyBaseURL+"/gettoken?"+query.Encode(), nil)
	if err != nil {
		return "", ErrDingTalkDirectoryFailed
	}
	resp, err := notifier.client().Do(req)
	if err != nil {
		return "", ErrDingTalkDirectoryFailed
	}
	defer resp.Body.Close()
	var result struct {
		ErrCode     int    `json:"errcode"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := common.DecodeJson(resp.Body, &result); err != nil || resp.StatusCode != http.StatusOK || result.ErrCode != 0 || result.AccessToken == "" {
		return "", ErrDingTalkDirectoryFailed
	}
	duration := time.Duration(result.ExpiresIn) * time.Second
	if duration > time.Minute {
		duration -= time.Minute
	}
	notifier.legacyAccessToken, notifier.legacyTokenExpiresAt = result.AccessToken, now.Add(duration)
	notifier.legacyTokenAppKey, notifier.legacyTokenAppSecret = appKey, appSecret
	return result.AccessToken, nil
}
