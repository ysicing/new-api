package system_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// DingTalkSettings 保存企业内部应用的扫码登录与机器人通知配置。
type DingTalkSettings struct {
	Enabled      bool   `json:"enabled"`
	CorpId       string `json:"corp_id"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RobotCode    string `json:"robot_code"`
}

var defaultDingTalkSettings = DingTalkSettings{}

func init() {
	config.GlobalConfig.Register("dingtalk", &defaultDingTalkSettings)
}

// GetDingTalkSettings 返回当前钉钉企业内部应用登录配置。
func GetDingTalkSettings() *DingTalkSettings {
	return &defaultDingTalkSettings
}

// IsRobotConfigured 仅在应用凭证和机器人编码齐全时启用机器人通知。
func (settings DingTalkSettings) IsRobotConfigured() bool {
	return strings.TrimSpace(settings.ClientId) != "" &&
		strings.TrimSpace(settings.ClientSecret) != "" &&
		strings.TrimSpace(settings.RobotCode) != ""
}
