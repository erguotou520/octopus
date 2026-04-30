package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bestruirui/octopus/internal/helper"
	"github.com/bestruirui/octopus/internal/model"
	"github.com/bestruirui/octopus/internal/op"
	"github.com/bestruirui/octopus/internal/relay/balancer"
	"github.com/bestruirui/octopus/internal/server/middleware"
	"github.com/bestruirui/octopus/internal/server/resp"
	"github.com/bestruirui/octopus/internal/server/router"
	"github.com/bestruirui/octopus/internal/task"
	transformerModel "github.com/bestruirui/octopus/internal/transformer/model"
	transformerOutbound "github.com/bestruirui/octopus/internal/transformer/outbound"
	"github.com/gin-gonic/gin"
)

func init() {
	router.NewGroupRouter("/api/v1/channel").
		Use(middleware.Auth()).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/list", http.MethodGet).
				Handle(listChannel),
		).
		AddRoute(
			router.NewRoute("/create", http.MethodPost).
				Handle(createChannel),
		).
		AddRoute(
			router.NewRoute("/update", http.MethodPost).
				Handle(updateChannel),
		).
		AddRoute(
			router.NewRoute("/enable", http.MethodPost).
				Handle(enableChannel),
		).
		AddRoute(
			router.NewRoute("/delete/:id", http.MethodDelete).
				Handle(deleteChannel),
		).
		AddRoute(
			router.NewRoute("/fetch-model", http.MethodPost).
				Handle(fetchModel),
		)
	router.NewGroupRouter("/api/v1/channel").
		Use(middleware.Auth()).
		AddRoute(
			router.NewRoute("/sync", http.MethodPost).
				Handle(syncChannel),
		).
		AddRoute(
			router.NewRoute("/last-sync-time", http.MethodGet).
				Handle(getLastSyncTime),
		).
		AddRoute(
			router.NewRoute("/test-models", http.MethodPost).
				Handle(testChannelModels),
		).
		AddRoute(
			router.NewRoute("/test-models-by-config", http.MethodPost).
				Handle(testChannelModelsByConfig),
		)
}

func listChannel(c *gin.Context) {
	channels, err := op.ChannelList(c.Request.Context())
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	for i, channel := range channels {
		stats := op.StatsChannelGet(channel.ID)
		channels[i].Stats = &stats
	}
	resp.Success(c, channels)
}

func createChannel(c *gin.Context) {
	var channel model.Channel
	if err := c.ShouldBindJSON(&channel); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if err := op.ChannelCreate(&channel, c.Request.Context()); err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	stats := op.StatsChannelGet(channel.ID)
	channel.Stats = &stats
	go func(channel *model.Channel) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		modelStr := channel.Model + "," + channel.CustomModel
		modelArray := strings.Split(modelStr, ",")
		helper.LLMPriceAddToDB(modelArray, ctx)
		helper.ChannelBaseUrlDelayUpdate(channel, ctx)
		helper.ChannelAutoGroup(channel, ctx)
	}(&channel)
	resp.Success(c, channel)
}

func updateChannel(c *gin.Context) {
	var req model.ChannelUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	channel, err := op.ChannelUpdate(&req, c.Request.Context())
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	balancer.ResetForChannel(channel.ID)
	stats := op.StatsChannelGet(channel.ID)
	channel.Stats = &stats
	go func(channel *model.Channel) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		modelStr := channel.Model + "," + channel.CustomModel
		modelArray := strings.Split(modelStr, ",")
		helper.LLMPriceAddToDB(modelArray, ctx)
		helper.ChannelBaseUrlDelayUpdate(channel, ctx)
		helper.ChannelAutoGroup(channel, ctx)
	}(channel)
	resp.Success(c, channel)
}

func enableChannel(c *gin.Context) {
	var request struct {
		ID      int  `json:"id"`
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if err := op.ChannelEnabled(request.ID, request.Enabled, c.Request.Context()); err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	balancer.ResetForChannel(request.ID)
	resp.Success(c, nil)
}

func deleteChannel(c *gin.Context) {
	id := c.Param("id")
	idNum, err := strconv.Atoi(id)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	if err := op.ChannelDel(idNum, c.Request.Context()); err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, nil)
}
func fetchModel(c *gin.Context) {
	var request model.Channel
	if err := c.ShouldBindJSON(&request); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	models, err := helper.FetchModels(c.Request.Context(), request)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, models)
}

func syncChannel(c *gin.Context) {
	task.SyncModelsTask()
	resp.Success(c, nil)
}

func getLastSyncTime(c *gin.Context) {
	time := task.GetLastSyncModelsTime()
	resp.Success(c, time)
}

func testChannelModels(c *gin.Context) {
	type TestModelRequest struct {
		ChannelID int      `json:"channel_id"`
		Models    []string `json:"models"`
	}
	type TestModelResult struct {
		Model  string `json:"model"`
		Passed bool   `json:"passed"`
		Error  string `json:"error,omitempty"`
		Delay  int    `json:"delay,omitempty"`
	}

	var req TestModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}

	if len(req.Models) == 0 {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}

	channel, err := op.ChannelGet(req.ChannelID, c.Request.Context())
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	results := make([]TestModelResult, 0, len(req.Models))

	for _, modelName := range req.Models {
		result := TestModelResult{Model: modelName}

		// 1. Base URL 连通性测试
		httpClient, err := helper.ChannelHttpClient(channel)
		if err != nil {
			result.Passed = false
			result.Error = "Failed to create HTTP client: " + err.Error()
			results = append(results, result)
			continue
		}

		baseURL := channel.GetBaseUrl()
		delay, err := helper.GetUrlDelay(httpClient, baseURL, c.Request.Context())
		if err != nil {
			result.Passed = false
			result.Error = "Connectivity test failed: " + err.Error()
			results = append(results, result)
			continue
		}
		result.Delay = delay

		// 2. LLM 调用测试
		content := "1+1=?"
		maxTokens := int64(1)
		temperature := 0.0
		testReq := transformerModel.InternalLLMRequest{
			Model:       modelName,
			Messages:    []transformerModel.Message{{Role: "user", Content: transformerModel.MessageContent{Content: &content}}},
			MaxTokens:   &maxTokens,
			Temperature: &temperature,
		}

		// 获取一个可用的 key
		channelKey := channel.GetChannelKey()
		if channelKey.ChannelKey == "" {
			result.Passed = false
			result.Error = "No available API key"
			results = append(results, result)
			continue
		}

		// 获取 outbound adapter
		outboundAdapter := transformerOutbound.GetForModel(transformerOutbound.OutboundType(channel.Type), modelName)
		if outboundAdapter == nil {
			result.Passed = false
			result.Error = "Unsupported channel type"
			results = append(results, result)
			continue
		}

		// 构建请求
		outboundReq, err := outboundAdapter.TransformRequest(c.Request.Context(), &testReq, baseURL, channelKey.ChannelKey)
		if err != nil {
			result.Passed = false
			result.Error = "Failed to build request: " + err.Error()
			results = append(results, result)
			continue
		}

		// 发送请求（设置超时）
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		resp, err := httpClient.Do(outboundReq.WithContext(ctx))
		if err != nil {
			result.Passed = false
			result.Error = "LLM request failed: " + err.Error()
			results = append(results, result)
			continue
		}
		defer resp.Body.Close()

		// 2xx 或 429（rate limited）均视为通过：请求格式正确，渠道可用
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			result.Passed = true
		} else if resp.StatusCode == http.StatusTooManyRequests {
			result.Passed = true
			result.Error = "Rate limited (429), but channel is reachable"
		} else {
			result.Passed = false
			result.Error = "LLM returned status: " + resp.Status
		}

		results = append(results, result)
	}

	c.JSON(http.StatusOK, results)
}

func testChannelModelsByConfig(c *gin.Context) {
	type TestModelByConfigRequest struct {
		Type     transformerOutbound.OutboundType `json:"type"`
		BaseUrls []model.BaseUrl                  `json:"base_urls"`
		Keys     []struct {
			Enabled    bool   `json:"enabled"`
			ChannelKey string `json:"channel_key"`
		} `json:"keys"`
		Proxy        bool                 `json:"proxy"`
		ChannelProxy *string              `json:"channel_proxy"`
		CustomHeader []model.CustomHeader `json:"custom_header"`
		Models       []string             `json:"models"`
	}
	type TestModelResult struct {
		Model  string `json:"model"`
		Passed bool   `json:"passed"`
		Error  string `json:"error,omitempty"`
		Delay  int    `json:"delay,omitempty"`
	}

	var req TestModelByConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}

	if len(req.Models) == 0 {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}

	// Build a temporary channel from config
	channel := &model.Channel{
		Type:         req.Type,
		BaseUrls:     req.BaseUrls,
		Proxy:        req.Proxy,
		ChannelProxy: req.ChannelProxy,
		CustomHeader: req.CustomHeader,
	}
	for _, k := range req.Keys {
		channel.Keys = append(channel.Keys, model.ChannelKey{
			Enabled:    k.Enabled,
			ChannelKey: k.ChannelKey,
		})
	}

	results := make([]TestModelResult, 0, len(req.Models))

	for _, modelName := range req.Models {
		result := TestModelResult{Model: modelName}

		httpClient, err := helper.ChannelHttpClient(channel)
		if err != nil {
			result.Passed = false
			result.Error = "Failed to create HTTP client: " + err.Error()
			results = append(results, result)
			continue
		}

		baseURL := channel.GetBaseUrl()
		delay, err := helper.GetUrlDelay(httpClient, baseURL, c.Request.Context())
		if err != nil {
			result.Passed = false
			result.Error = "Connectivity test failed: " + err.Error()
			results = append(results, result)
			continue
		}
		result.Delay = delay

		content := "1+1=?"
		maxTokens := int64(1)
		temperature := 0.0
		testReq := transformerModel.InternalLLMRequest{
			Model:       modelName,
			Messages:    []transformerModel.Message{{Role: "user", Content: transformerModel.MessageContent{Content: &content}}},
			MaxTokens:   &maxTokens,
			Temperature: &temperature,
		}

		channelKey := channel.GetChannelKey()
		if channelKey.ChannelKey == "" {
			result.Passed = false
			result.Error = "No available API key"
			results = append(results, result)
			continue
		}

		outboundAdapter := transformerOutbound.GetForModel(transformerOutbound.OutboundType(req.Type), modelName)
		if outboundAdapter == nil {
			result.Passed = false
			result.Error = "Unsupported channel type"
			results = append(results, result)
			continue
		}

		outboundReq, err := outboundAdapter.TransformRequest(c.Request.Context(), &testReq, baseURL, channelKey.ChannelKey)
		if err != nil {
			result.Passed = false
			result.Error = "Failed to build request: " + err.Error()
			results = append(results, result)
			continue
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		httpResp, err := httpClient.Do(outboundReq.WithContext(ctx))
		if err != nil {
			result.Passed = false
			result.Error = "LLM request failed: " + err.Error()
			results = append(results, result)
			continue
		}
		defer httpResp.Body.Close()

		// 2xx 或 429（rate limited）均视为通过：请求格式正确，渠道可用
		if httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {
			result.Passed = true
		} else if httpResp.StatusCode == http.StatusTooManyRequests {
			result.Passed = true
			result.Error = "Rate limited (429), but channel is reachable"
		} else {
			result.Passed = false
			result.Error = "LLM returned status: " + httpResp.Status
		}

		results = append(results, result)
	}

	c.JSON(http.StatusOK, results)
}
