package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// anthropicVersion is required by api.anthropic.com; other providers are not
// sent it, because a few of them reject headers they do not know.
const anthropicVersion = "2023-06-01"

// oauthBeta is what Claude Code sends beside a setup token. The model listing
// answers without it, but keeping the header makes the two call sites match
// what Claude Code itself does.
const oauthBeta = "oauth-2025-04-20"

func oauthGet(url, token string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("anthropic-beta", oauthBeta)

	client := &http.Client{Timeout: 15 * time.Second}
	return client.Do(req)
}

// CheckOAuth validates a setup token against the model listing.
//
// It deliberately does not send an inference request. A messages call spends
// subscription quota, and a rate-limited account answers 429 to a token that is
// perfectly valid — which reads as "bad key" and sends the user chasing the
// wrong problem. A 200 here means the token authenticates; quota is somebody
// else's question.
func CheckOAuth(baseURL, token string) error {
	url := strings.TrimRight(baseURL, "/") + "/v1/models?limit=1"
	resp, err := oauthGet(url, token)
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
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&errBody)
	if errBody.Error.Message != "" {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, errBody.Error.Message)
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}

// FetchModelsOAuth lists the account's models with a setup token. The listing
// is the only read that both validates the token and names the models, which is
// why the setup wizard uses it instead of a ping.
func FetchModelsOAuth(baseURL, modelsURL, token string) ([]ModelInfo, error) {
	searchURL := modelsURL
	if searchURL == "" {
		searchURL = baseURL
	}
	for _, candidate := range modelListURLs(searchURL) {
		models, err := fetchOAuth(candidate, token)
		if err != nil {
			continue
		}
		return models, nil
	}
	return nil, fmt.Errorf("could not fetch models from any endpoint")
}

func fetchOAuth(url, token string) ([]ModelInfo, error) {
	resp, err := oauthGet(url, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return payload.Data, nil
}
