package helper

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bestruirui/octopus/internal/model"
	"github.com/dlclark/regexp2"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

func FetchModels(ctx context.Context, request model.Channel) ([]string, error) {
	client, err := ChannelHttpClient(&request)
	if err != nil {
		return nil, err
	}
	fetchModel := make([]string, 0)
	switch request.Type {
	case llm.APIFormatAnthropicMessage:
		fetchModel, err = fetchAnthropicModels(client, ctx, request)
	case llm.APIFormatGeminiContents:
		fetchModel, err = fetchGeminiModels(client, ctx, request)
	case model.ChannelTypeAntigravity:
		fetchModel, err = fetchAntigravityModels(client, ctx, request)
	default:
		fetchModel, err = fetchOpenAIModels(client, ctx, request)
	}
	if err != nil {
		return nil, err
	}
	if request.MatchRegex != nil && *request.MatchRegex != "" {
		matchModel := make([]string, 0)
		re, err := regexp2.Compile(*request.MatchRegex, regexp2.ECMAScript)
		if err != nil {
			return nil, err
		}
		for _, model := range fetchModel {
			matched, err := re.MatchString(model)
			if err != nil {
				return nil, err
			}
			if matched {
				matchModel = append(matchModel, model)
			}
		}
		return matchModel, nil
	}
	return fetchModel, nil
}

// refer: https://platform.openai.com/docs/api-reference/models/list
func fetchOpenAIModels(client *http.Client, ctx context.Context, request model.Channel) ([]string, error) {
	baseURL := transformer.NormalizeBaseURL(request.GetBaseUrl(), "v1")
	if request.Type == model.ChannelTypeDoubao {
		baseURL = transformer.NormalizeBaseURL(request.GetBaseUrl(), "v3")
	}
	req, _ := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		baseURL+"/models",
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+request.GetChannelKey().ChannelKey)
	applyCustomHeaders(req, request)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result model.OpenAIModelList

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

// refer: https://ai.google.dev/api/models
func fetchGeminiModels(client *http.Client, ctx context.Context, request model.Channel) ([]string, error) {
	var allModels []string
	pageToken := ""
	baseURL := transformer.NormalizeBaseURL(request.GetBaseUrl(), "v1beta")
	// Gemini transformer 会保留用户显式填写的 /v1；这里同样处理，避免把 /v1 拼成 /v1/v1beta。
	if strings.HasSuffix(strings.TrimRight(request.GetBaseUrl(), "/"), "/v1") {
		baseURL = transformer.NormalizeBaseURL(request.GetBaseUrl(), "")
	}

	for {
		req, _ := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			baseURL+"/models",
			nil,
		)
		req.Header.Set("X-Goog-Api-Key", request.GetChannelKey().ChannelKey)
		applyCustomHeaders(req, request)
		if pageToken != "" {
			q := req.URL.Query()
			q.Add("pageToken", pageToken)
			req.URL.RawQuery = q.Encode()
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var result model.GeminiModelList

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		for _, m := range result.Models {
			name := strings.TrimPrefix(m.Name, "models/")
			allModels = append(allModels, name)
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}
	if len(allModels) == 0 {
		return fetchOpenAIModels(client, ctx, request)
	}
	return allModels, nil
}

// refer: https://platform.claude.com/docs
func fetchAnthropicModels(client *http.Client, ctx context.Context, request model.Channel) ([]string, error) {
	var allModels []string
	var afterID string
	baseURL := transformer.NormalizeBaseURL(request.GetBaseUrl(), "v1")
	for {

		req, _ := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			baseURL+"/models",
			nil,
		)
		req.Header.Set("X-Api-Key", request.GetChannelKey().ChannelKey)
		req.Header.Set("Anthropic-Version", "2023-06-01")
		applyCustomHeaders(req, request)
		// 设置多页参数
		q := req.URL.Query()

		if afterID != "" {
			q.Set("after_id", afterID)
		}
		req.URL.RawQuery = q.Encode()

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var result model.AnthropicModelList

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		for _, m := range result.Data {
			allModels = append(allModels, m.ID)
		}

		if !result.HasMore {
			break
		}

		afterID = result.LastID
	}
	if len(allModels) == 0 {
		return fetchOpenAIModels(client, ctx, request)
	}
	return allModels, nil
}

func applyCustomHeaders(req *http.Request, channel model.Channel) {
	for _, header := range channel.CustomHeader {
		if header.HeaderKey != "" {
			req.Header.Set(header.HeaderKey, header.HeaderValue)
		}
	}
}

// fetchAntigravityModels retrieves models for Antigravity (Google Gemini Code Assist via OAuth).
// It calls POST /v1internal:retrieveUserQuota to get quota buckets, each containing a modelId.
// Key format: "<oauth_token>" or "<oauth_token>|<projectId>"
func fetchAntigravityModels(client *http.Client, ctx context.Context, request model.Channel) ([]string, error) {
	key := request.GetChannelKey().ChannelKey
	keyParts := strings.SplitN(key, "|", 2)
	token := keyParts[0]

	projectID := ""
	if len(keyParts) == 2 {
		projectID = keyParts[1]
	} else {
		loadBody := `{"metadata":{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}`
		loadReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
			request.GetBaseUrl()+"/v1internal:loadCodeAssist",
			strings.NewReader(loadBody))
		if err != nil {
			return nil, err
		}
		loadReq.Header.Set("Authorization", "Bearer "+token)
		loadReq.Header.Set("Content-Type", "application/json")
		loadReq.Header.Set("X-Goog-Api-Client", "gl-node/22.17.0")
		loadReq.Header.Set("Client-Metadata", "ideType=IDE_UNSPECIFIED,platform=PLATFORM_UNSPECIFIED,pluginType=GEMINI")

		loadResp, err := client.Do(loadReq)
		if err != nil {
			return nil, err
		}
		defer loadResp.Body.Close()

		var loadPayload struct {
			CloudAiCompanionProject interface{} `json:"cloudaicompanionProject"`
		}
		if err := json.NewDecoder(loadResp.Body).Decode(&loadPayload); err != nil {
			return nil, err
		}
		switch v := loadPayload.CloudAiCompanionProject.(type) {
		case string:
			projectID = strings.TrimSpace(v)
		case map[string]interface{}:
			if id, ok := v["id"].(string); ok {
				projectID = strings.TrimSpace(id)
			}
		}
	}

	quotaBody := `{"project":"` + projectID + `"}`
	quotaReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		request.GetBaseUrl()+"/v1internal:retrieveUserQuota",
		strings.NewReader(quotaBody))
	if err != nil {
		return nil, err
	}
	quotaReq.Header.Set("Authorization", "Bearer "+token)
	quotaReq.Header.Set("Content-Type", "application/json")
	quotaReq.Header.Set("X-Goog-Api-Client", "gl-node/22.17.0")
	quotaReq.Header.Set("Client-Metadata", "ideType=IDE_UNSPECIFIED,platform=PLATFORM_UNSPECIFIED,pluginType=GEMINI")
	applyCustomHeaders(quotaReq, request)

	quotaResp, err := client.Do(quotaReq)
	if err != nil {
		return nil, err
	}
	defer quotaResp.Body.Close()

	var quotaPayload struct {
		Buckets []struct {
			ModelID string `json:"modelId"`
		} `json:"buckets"`
	}
	if err := json.NewDecoder(quotaResp.Body).Decode(&quotaPayload); err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var models []string
	for _, bucket := range quotaPayload.Buckets {
		if bucket.ModelID != "" && !seen[bucket.ModelID] {
			seen[bucket.ModelID] = true
			models = append(models, bucket.ModelID)
		}
	}
	return models, nil
}
