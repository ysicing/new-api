package service

import (
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTextOtherInfoIncludesResponseModel(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		StartTime:         time.Unix(100, 0),
		FirstResponseTime: time.Unix(101, 0),
		ChannelMeta:       &relaycommon.ChannelMeta{},
		ResponseModel: &relaycommon.ResponseModel{
			RequestedModel: "gpt-6-astra",
			UpstreamModel:  "mapped-gpt-6-astra",
			ReturnedModel:  "gpt-5.6-luna",
		},
	}

	other := GenerateTextOtherInfo(context, info, 1, 1, 1, 0, 0, 0, 1)
	responseModel, ok := other["response_model"].(*relaycommon.ResponseModel)

	require.True(t, ok)
	assert.Equal(t, "gpt-6-astra", responseModel.RequestedModel)
	assert.Equal(t, "mapped-gpt-6-astra", responseModel.UpstreamModel)
	assert.Equal(t, "gpt-5.6-luna", responseModel.ReturnedModel)
}
