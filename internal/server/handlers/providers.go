package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/bestruirui/octopus/internal/conf"
	"github.com/bestruirui/octopus/internal/model"
	"github.com/bestruirui/octopus/internal/server/router"
	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/llm"
)

const providersGitHubURL = "https://raw.githubusercontent.com/erguotou520/octopus/dev/providers.json"

// localProvidersFile is the path to the local providers.json file (used in dev mode)
const localProvidersFile = "providers.json"

type Provider struct {
	Name        string         `json:"name"`
	ChannelType llm.APIFormat  `json:"channel_type"`
	BaseURL     string         `json:"base_url"`
}

// defaultProviders is the fallback list when all other sources are unavailable
var defaultProviders = []Provider{
	{Name: "OpenAI", ChannelType: llm.APIFormatOpenAIChatCompletion, BaseURL: "https://api.openai.com/v1"},
	{Name: "Anthropic", ChannelType: llm.APIFormatAnthropicMessage, BaseURL: "https://api.anthropic.com/v1"},
	{Name: "Gemini", ChannelType: llm.APIFormatGeminiContents, BaseURL: "https://generativelanguage.googleapis.com/v1beta"},
	{Name: "火山引擎", ChannelType: model.ChannelTypeDoubao, BaseURL: "https://ark.cn-beijing.volces.com/api/v3"},
	{Name: "OpenAI Embedding", ChannelType: llm.APIFormatOpenAIEmbedding, BaseURL: "https://api.openai.com/v1"},
	{Name: "OpenRouter", ChannelType: llm.APIFormatOpenAIChatCompletion, BaseURL: "https://openrouter.ai/api/v1"},
	{Name: "质谱 AI", ChannelType: llm.APIFormatOpenAIChatCompletion, BaseURL: "https://open.bigmodel.cn/api/paas/v4"},
	{Name: "质谱 Coding Plan", ChannelType: llm.APIFormatOpenAIChatCompletion, BaseURL: "https://open.bigmodel.cn/api/coding/paas/v4"},
	{Name: "GitHub Copilot", ChannelType: model.ChannelTypeGithubCopilot, BaseURL: "https://api.githubcopilot.com"},
	{Name: "Vercel AI Gateway", ChannelType: llm.APIFormatOpenAIChatCompletion, BaseURL: "https://ai.vercel.app/api/v1"},
	{Name: "OpenCode Zen", ChannelType: model.ChannelTypeZen, BaseURL: "https://opencode.ai/zen/v1"},
	{Name: "Antigravity", ChannelType: model.ChannelTypeAntigravity, BaseURL: "https://generativelanguage.googleapis.com/v1beta"},
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
