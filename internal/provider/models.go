package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FetchModels tries to get the current model list from a provider's /v1/models
// endpoint. It first tries Bearer auth, then falls back to X-Api-Key (MiniMax
// style). Returns nil, error if both fail or no models found — callers should
// fall back to manual model entry.
func FetchModels(baseURL, apiKey string) ([]ModelInfo, error) {
	url := baseURL + "/v1/models"

	// Try Bearer auth first (Anthropic, OpenRouter, DeepSeek, MaaS).
	models, err := fetchWithAuth(url, "Bearer "+apiKey)
	if err == nil {
		return models, nil
	}

	// Fall back to X-Api-Key (MiniMax style).
	return fetchWithAuth(url, apiKey)
}

func fetchWithAuth(url, authValue string) ([]ModelInfo, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if authValue != "" {
		req.Header.Set("Authorization", authValue)
		req.Header.Set("X-Api-Key", authValue)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: could not fetch models for this provider", resp.StatusCode)
	}

	// Limit read to prevent unbounded memory from provider-controlled response.
	limited := io.LimitReader(resp.Body, 1<<20) // 1MB max
	var result struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(limited).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse models: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("provider returned no models — the API key may not grant model access")
	}

	return result.Data, nil
}
