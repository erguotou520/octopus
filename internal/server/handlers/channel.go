package handlers

import (
	"context"
	"fmt"
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
	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/llm"
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

func testChannelModelsByConfig(c *gin.Context) {
	type TestModelResult struct {
		Model  string `json:"model"`
		Passed bool   `json:"passed"`
		Error  string `json:"error,omitempty"`
		Delay  int    `json:"delay,omitempty"`
	}

	var req struct {
		Type         llm.APIFormat        `json:"type"`
		BaseUrls     []model.BaseUrl      `json:"base_urls"`
		Keys         []model.ChannelKey   `json:"keys"`
		Proxy        bool                 `json:"proxy"`
		ChannelProxy *string              `json:"channel_proxy"`
		CustomHeader []model.CustomHeader `json:"custom_header"`
		Models       []string             `json:"models"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if len(req.Models) == 0 {
		resp.Error(c, http.StatusBadRequest, "models is required")
		return
	}

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

	httpClient, err := helper.ChannelHttpClient(channel)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, "failed to create HTTP client: "+err.Error())
		return
	}

	results := make([]TestModelResult, 0, len(req.Models))
	for _, modelName := range req.Models {
		result := TestModelResult{Model: modelName}

		baseURL := channel.GetBaseUrl()
		delay, delayErr := helper.GetUrlDelay(httpClient, baseURL, c.Request.Context())
		if delayErr != nil {
			result.Passed = false
			result.Error = "connectivity test failed: " + delayErr.Error()
			results = append(results, result)
			continue
		}
		result.Delay = delay

		key := channel.GetChannelKey()
		if key.ChannelKey == "" {
			result.Passed = false
			result.Error = "no available API key"
			results = append(results, result)
			continue
		}

		testBody := fmt.Sprintf(`{"model":"%s","messages":[{"role":"user","content":"1+1=?"}],"max_tokens":1}`, modelName)
		testReq, reqErr := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, baseURL+"/chat/completions", strings.NewReader(testBody))
		if reqErr != nil {
			result.Passed = false
			result.Error = "failed to create request: " + reqErr.Error()
			results = append(results, result)
			continue
		}
		testReq.Header.Set("Content-Type", "application/json")
		testReq.Header.Set("Authorization", "Bearer "+key.ChannelKey)
		for _, h := range channel.CustomHeader {
			if h.HeaderKey != "" {
				testReq.Header.Set(h.HeaderKey, h.HeaderValue)
			}
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		testReq = testReq.WithContext(ctx)

		resp2, httpErr := httpClient.Do(testReq)
		if httpErr != nil {
			result.Passed = false
			result.Error = "LLM request failed: " + httpErr.Error()
			results = append(results, result)
			continue
		}
		resp2.Body.Close()

		if resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
			result.Passed = true
		} else if resp2.StatusCode == http.StatusTooManyRequests {
			result.Passed = true
			result.Error = "Rate limited (429), but channel is reachable"
		} else {
			result.Passed = false
			result.Error = "LLM returned status: " + resp2.Status
		}

		results = append(results, result)
	}

	c.JSON(http.StatusOK, results)
}
