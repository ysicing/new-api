package common

import "strings"

// ResponseModel records the model names declared by the request, channel, and
// upstream response for diagnostic display only.
type ResponseModel struct {
	RequestedModel string `json:"requested_model"`
	UpstreamModel  string `json:"upstream_model"`
	ReturnedModel  string `json:"returned_model"`
}

func (model *ResponseModel) matches(returnedModel string) bool {
	returned := strings.ToLower(returnedModel)
	for _, expected := range []string{model.RequestedModel, model.UpstreamModel} {
		expected = strings.ToLower(expected)
		if expected != "" && (strings.HasPrefix(returned, expected) || strings.HasSuffix(returned, expected)) {
			return true
		}
	}
	return false
}

// Mismatch reports whether the upstream response model differs from both
// models the gateway knows for this request.
func (model *ResponseModel) Mismatch() bool {
	return model != nil && strings.TrimSpace(model.ReturnedModel) != "" && !model.matches(model.ReturnedModel)
}

// ObserveResponseModel records the first upstream-declared model that differs
// from a previous mismatch. Synthesized converter response models must not call
// this method.
func (info *RelayInfo) ObserveResponseModel(returnedModel string) {
	if info == nil || strings.TrimSpace(returnedModel) == "" {
		return
	}
	if info.ResponseModel == nil {
		info.ResponseModel = &ResponseModel{
			RequestedModel: info.OriginModelName,
			UpstreamModel:  info.GetUpstreamModelName(),
		}
	}
	if info.ResponseModel.Mismatch() {
		return
	}
	if info.ResponseModel.matches(returnedModel) && info.ResponseModel.ReturnedModel != "" {
		return
	}
	info.ResponseModel.ReturnedModel = returnedModel
}
