package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObserveResponseModelDetectsUpstreamMismatch(t *testing.T) {
	tests := []struct {
		name     string
		returned string
		mismatch bool
	}{
		{name: "requested model", returned: "gpt-6-astra", mismatch: false},
		{name: "dated requested model", returned: "gpt-6-astra-20260918", mismatch: false},
		{name: "provider path", returned: "vendor/gpt-6-astra", mismatch: false},
		{name: "different model", returned: "gpt-5.6-luna", mismatch: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := &RelayInfo{
				OriginModelName: "gpt-6-astra",
				ChannelMeta: &ChannelMeta{
					UpstreamModelName: "mapped-gpt-6-astra",
				},
			}

			info.ObserveResponseModel(test.returned)

			require.NotNil(t, info.ResponseModel)
			assert.Equal(t, "gpt-6-astra", info.ResponseModel.RequestedModel)
			assert.Equal(t, "mapped-gpt-6-astra", info.ResponseModel.UpstreamModel)
			assert.Equal(t, test.returned, info.ResponseModel.ReturnedModel)
			assert.Equal(t, test.mismatch, info.ResponseModel.Mismatch())
		})
	}
}

func TestObserveResponseModelKeepsTheFirstMismatch(t *testing.T) {
	info := &RelayInfo{
		OriginModelName: "gpt-6-astra",
		ChannelMeta: &ChannelMeta{
			UpstreamModelName: "mapped-gpt-6-astra",
		},
	}

	info.ObserveResponseModel("gpt-5.6-luna")
	info.ObserveResponseModel("gpt-4o")

	require.NotNil(t, info.ResponseModel)
	assert.Equal(t, "gpt-5.6-luna", info.ResponseModel.ReturnedModel)
	assert.True(t, info.ResponseModel.Mismatch())
}
