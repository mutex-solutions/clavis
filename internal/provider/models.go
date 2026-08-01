package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// FetchModels calls GET /v1/models on the provider endpoint to get the current
// model list. Requires a valid API key.
func FetchModels(baseURL, apiKey string) ([]ModelInfo, error) {
	url := baseURL + "/v1/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: could not fetch models for this provider", resp.StatusCode)
	}

	var result struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse models: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("provider returned no models — the API key may not grant model access")
	}

	return result.Data, nil
}
