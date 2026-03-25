package appkey

import "encoding/json"

// usagePayload is a minimal struct for extracting token counts from a
// provider response body without allocating a full response object.
type usagePayload struct {
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

// ExtractUsage parses promptTokens and completionTokens from a JSON response
// body. Returns zeros if the fields are absent or the body is not valid JSON.
// Intended to be called in a background goroutine — correctness over speed.
func ExtractUsage(body []byte) (promptTokens, completionTokens int64) {
	if len(body) == 0 {
		return 0, 0
	}
	var p usagePayload
	if err := json.Unmarshal(body, &p); err != nil {
		return 0, 0
	}
	return p.Usage.PromptTokens, p.Usage.CompletionTokens
}
