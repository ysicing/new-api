package deepseek

import "github.com/QuantumNous/new-api/constant"

var ModelList = []string{
	"deepseek-chat", "deepseek-reasoner",
	"deepseek-flash", "deepseek-flash-none", "deepseek-flash-low", "deepseek-flash-max",
	"deepseek-v4-flash", "deepseek-v4-flash-none", "deepseek-v4-flash-low", "deepseek-v4-flash-max",
	"deepseek-v4-pro", "deepseek-v4-pro-none", "deepseek-v4-pro-low", "deepseek-v4-pro-max",
	constant.DeepSeekSearchModel, constant.DeepSeekFetchModel,
}

var ChannelName = "deepseek"

const (
	SearchToolPriceKey = "deepseek_search"
	FetchToolPriceKey  = "deepseek_fetch"
)
