package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/bestruirui/octopus/internal/conf"
	"github.com/bestruirui/octopus/internal/server/router"
	"github.com/gin-gonic/gin"
)

const providersGitHubURL = "https://raw.githubusercontent.com/erguotou520/octopus/dev/providers.json"

// localProvidersFile is the path to the local providers.json file (used in dev mode)
const localProvidersFile = "providers.json"

type Provider struct {
	Name        string `json:"name"`
	ChannelType int    `json:"channel_type"`
	BaseURL     string `json:"base_url"`
}

// defaultProviders is the fallback list when all other sources are unavailable
var defaultProviders = []Provider{
	{Name: "OpenAI", ChannelType: 0, BaseURL: "https://api.openai.com/v1"},
	{Name: "Anthropic", ChannelType: 2, BaseURL: "https://api.anthropic.com/v1"},
	{Name: "Gemini", ChannelType: 3, BaseURL: "https://generativelanguage.googleapis.com/v1beta"},
	{Name: "火山引擎", ChannelType: 4, BaseURL: "https://ark.cn-beijing.volces.com/api/v3"},
	{Name: "OpenAI Embedding", ChannelType: 5, BaseURL: "https://api.openai.com/v1"},
	{Name: "OpenRouter", ChannelType: 0, BaseURL: "https://openrouter.ai/api/v1"},
	{Name: "质谱 AI", ChannelType: 0, BaseURL: "https://open.bigmodel.cn/api/paas/v4"},
	{Name: "质谱 Coding Plan", ChannelType: 0, BaseURL: "https://open.bigmodel.cn/api/coding/paas/v4"},
	{Name: "GitHub Copilot", ChannelType: 6, BaseURL: "https://api.githubcopilot.com"},
	{Name: "Vercel AI Gateway", ChannelType: 0, BaseURL: "https://ai.vercel.app/api/v1"},
	{Name: "OpenCode Zen", ChannelType: 8, BaseURL: "https://opencode.ai/zen/v1"},
	{Name: "Antigravity", ChannelType: 7, BaseURL: "https://generativelanguage.googleapis.com/v1beta"},
}

// readLocalProviders reads providers from the local providers.json file
func readLocalProviders() ([]Provider, error) {
	data, err := os.ReadFile(localProvidersFile)
	if err != nil {
		return nil, err
	}
	var providers []Provider
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, err
	}
	return providers, nil
}

// GetProviders returns the list of providers
// Dev mode: local providers.json → fallback to default
// Production mode: GitHub raw URL → local providers.json → fallback to default
func GetProviders(c *gin.Context) {
	// In debug/dev mode, load from local file first
	if conf.IsDebug() {
		if providers, err := readLocalProviders(); err == nil {
			c.JSON(http.StatusOK, providers)
			return
		}
		// Dev mode fallback to default
		c.JSON(http.StatusOK, defaultProviders)
		return
	}

	// Production mode: try fetching from GitHub first (with timeout)
	var providers []Provider
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(providersGitHubURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		if jsonErr := json.NewDecoder(resp.Body).Decode(&providers); jsonErr == nil {
			c.JSON(http.StatusOK, providers)
			return
		}
	}

	// Try local file as second fallback
	if providers, err := readLocalProviders(); err == nil {
		c.JSON(http.StatusOK, providers)
		return
	}

	// Final fallback to default providers
	c.JSON(http.StatusOK, defaultProviders)
}

func init() {
	router.NewGroupRouter("/api/v1/providers").
		AddRoute(
			router.NewRoute("", http.MethodGet).Handle(GetProviders),
		)
}
