package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FetchModels tries to get the current model list from a provider.
// Strategy: try baseURL/v1/models first, then strip the path prefix and try
// again at the root (e.g., api.deepseek.com/v1/models when the anthropic
// endpoint is api.deepseek.com/anthropic/v1/messages).
func FetchModels(baseURL, apiKey string) ([]ModelInfo, error) {
	urls := modelListURLs(baseURL)

	for _, candidate := range urls {
		models, err := fetch(candidate, apiKey)
		if err == nil {
			return models, nil
		}
	}
	return nil, fmt.Errorf("could not fetch models from any endpoint")
}

// modelListURLs returns candidate /v1/models URLs to try.
func modelListURLs(baseURL string) []string {
	cleaned := strings.TrimRight(baseURL, "/")
	candidates := []string{cleaned + "/v1/models"}

	// If baseURL has a path prefix (e.g. /anthropic), also try the root.
	if u, err := url.Parse(cleaned); err == nil && u.Path != "" && u.Path != "/" {
		u.Path = "/v1/models"
		candidates = append(candidates, u.String())
	}
	return candidates
}

func fetch(url, apiKey string) ([]ModelInfo, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-Api-Key", apiKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, 1<<20)
	var result struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(limited).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse models: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("provider returned no models")
	}

	return result.Data, nil
}
