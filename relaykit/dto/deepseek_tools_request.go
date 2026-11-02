package dto

import (
	"encoding/json"
	"net/http"

	"github.com/QuantumNous/new-api/relaykit/types"
)

type DeepSeekToolsRequest struct {
	Model   string          `json:"model,omitempty"`
	RawBody json.RawMessage `json:"-"`
}

func (request *DeepSeekToolsRequest) GetTokenCountMeta() *types.TokenCountMeta {
	return &types.TokenCountMeta{CombineText: string(request.RawBody), TokenType: types.TokenTypeTokenizer}
}

func (request *DeepSeekToolsRequest) IsStream(_ *http.Request) bool {
	return false
}

func (request *DeepSeekToolsRequest) SetModelName(modelName string) {
	request.Model = modelName
}
