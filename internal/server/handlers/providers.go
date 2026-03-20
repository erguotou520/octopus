package handlers

import (
	"io"
	"net/http"
	"time"

	"github.com/bestruirui/octopus/internal/assets"
	"github.com/bestruirui/octopus/internal/server/router"
	"github.com/bestruirui/octopus/internal/utils/log"
	"github.com/gin-gonic/gin"
)

const providersGitHubURL = "https://raw.githubusercontent.com/erguotou520/octopus/refs/heads/feat/erguotou/internal/assets/providers.json"

// GetProviders returns the list of providers
// Priority: GitHub raw URL → embedded providers.json
func GetProviders(c *gin.Context) {
	// Try fetching from GitHub first (with timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	log.Infof("Fetching providers from GitHub: %s", providersGitHubURL)
	resp, err := client.Get(providersGitHubURL)
	if err != nil {
		log.Warnf("Failed to fetch providers from GitHub: %s, error: %v", providersGitHubURL, err)
	} else if resp.StatusCode != http.StatusOK {
		log.Warnf("GitHub returned non-OK status: %d for %s", resp.StatusCode, providersGitHubURL)
		resp.Body.Close()
	} else {
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err == nil {
			c.Data(resp.StatusCode, "application/json", data)
			log.Infof("Successfully fetched providers from GitHub")
			return
		}
		log.Warnf("Failed to read response body from GitHub: %v", err)
	}

	// Fallback to embedded providers.json
	log.Infof("Loading providers from embedded providers.json")
	file, err := assets.ProvidersFS.Open("providers.json")
	if err != nil {
		log.Errorf("Failed to open embedded providers.json: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load providers"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		log.Errorf("Failed to read embedded providers.json: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load providers"})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}

func init() {
	router.NewGroupRouter("/api/v1/providers").
		AddRoute(
			router.NewRoute("", http.MethodGet).Handle(GetProviders),
		)
}
