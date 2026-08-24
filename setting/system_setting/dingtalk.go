package system_setting

import "github.com/QuantumNous/new-api/setting/config"

// DingTalkSettings 保存企业内部应用扫码登录配置。
type DingTalkSettings struct {
	Enabled      bool   `json:"enabled"`
	CorpId       string `json:"corp_id"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

var defaultDingTalkSettings = DingTalkSettings{}

func init() {
	config.GlobalConfig.Register("dingtalk", &defaultDingTalkSettings)
}

// GetDingTalkSettings 返回当前钉钉企业内部应用登录配置。
func GetDingTalkSettings() *DingTalkSettings {
	return &defaultDingTalkSettings
}
