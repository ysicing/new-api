package constant

import "github.com/QuantumNous/new-api/relaykit/types"

// EndpointType moved to types with the conversion kit; aliases keep host
// code compiling unchanged.
type EndpointType = types.EndpointType

const (
	EndpointTypeOpenAI                = types.EndpointTypeOpenAI
	EndpointTypeOpenAIResponse        = types.EndpointTypeOpenAIResponse
	EndpointTypeOpenAIResponseCompact = types.EndpointTypeOpenAIResponseCompact
	EndpointTypeOpenAIAlphaSearch     = types.EndpointTypeOpenAIAlphaSearch
	EndpointTypeDeepSeekToolsSearch   = types.EndpointTypeDeepSeekToolsSearch
	EndpointTypeDeepSeekToolsFetch    = types.EndpointTypeDeepSeekToolsFetch
	EndpointTypeAnthropic             = types.EndpointTypeAnthropic
	EndpointTypeGemini                = types.EndpointTypeGemini
	EndpointTypeDecisions             = types.EndpointTypeDecisions
	EndpointTypeJinaRerank            = types.EndpointTypeJinaRerank
	EndpointTypeImageGeneration       = types.EndpointTypeImageGeneration
	EndpointTypeEmbeddings            = types.EndpointTypeEmbeddings
	EndpointTypeOpenAIVideo           = types.EndpointTypeOpenAIVideo
)

const (
	DeepSeekSearchModel = "deepseek-search"
	DeepSeekFetchModel  = "deepseek-fetch"
)
