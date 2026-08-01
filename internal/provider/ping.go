package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Ping sends a minimal request to a provider's Messages API to validate the
// key and endpoint. Tries Bearer auth first, then X-Api-Key (MiniMax style).
// Returns nil on success (HTTP 2xx), or an error.
func Ping(baseURL, apiKey, model string) error {
	// Normalize: strip trailing slashes to prevent // in URL.
	url := strings.TrimRight(baseURL, "/") + "/v1/messages"

	body := map[string]interface{}{
		"model":      model,
		"max_tokens": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	// Try Bearer first, then X-Api-Key.
	for _, authVal := range []string{"Bearer " + apiKey, apiKey} {
		err = pingWithAuth(url, data, authVal)
		if err == nil {
			return nil
		}
	}
	return err
}

func pingWithAuth(url string, body []byte, authValue string) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if authValue != "" {
		req.Header.Set("Authorization", authValue)
		req.Header.Set("X-Api-Key", authValue)
	}
	// Do NOT send anthropic-version — it's Anthropic-specific and 3P providers
	// like MiniMax, Z.ai, MaaS may reject it.

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var errBody struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&errBody)

	if errBody.Error.Message != "" {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, errBody.Error.Message)
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}
