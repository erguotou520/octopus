package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/bestruirui/octopus/internal/server/router"
	"github.com/gin-gonic/gin"
)

const providersGitHubURL = "https://raw.githubusercontent.com/bestruirui/octopus/dev/providers.json"

type Provider struct {
	Name        string `json:"name"`
	ChannelType int    `json:"channel_type"`
	BaseURL     string `json:"base_url"`
}

// defaultProviders is the fallback list when GitHub is unavailable
var defaultProviders = []Provider{
	{Name: "OpenAI", ChannelType: 0, BaseURL: "https://api.openai.com/v1"},
	{Name: "OpenAI Response", ChannelType: 1, BaseURL: "https://api.openai.com/v1"},
	{Name: "Anthropic", ChannelType: 2, BaseURL: "https://api.anthropic.com/v1"},
	{Name: "Gemini", ChannelType: 3, BaseURL: "https://generativelanguage.googleapis.com/v1beta"},
	{Name: "Volcengine", ChannelType: 4, BaseURL: "https://ark.cn-beijing.volces.com/api/v3"},
	{Name: "OpenAI Embedding", ChannelType: 5, BaseURL: "https://api.openai.com/v1"},
}

// GetProviders returns the list of providers
// Priority: GitHub raw URL > default providers (hardcoded)
func GetProviders(c *gin.Context) {
	var providers []Provider

	// Try fetching from GitHub first (with timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(providersGitHubURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		if jsonErr := json.NewDecoder(resp.Body).Decode(&providers); jsonErr == nil {
			c.JSON(http.StatusOK, providers)
			return
		}
	}

	// Fallback to default providers
	c.JSON(http.StatusOK, defaultProviders)
}

func init() {
	router.NewGroupRouter("/api/v1/providers").
		AddRoute(
			router.NewRoute("", http.MethodGet).Handle(GetProviders),
		)
}
