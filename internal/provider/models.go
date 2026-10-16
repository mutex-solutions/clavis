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

// anthropicVersionHeader asks a gateway for its Anthropic-shaped model listing.
//
// Some gateways answer /v1/models with a different body depending on the request
// headers: an OpenAI-shaped listing, often trimmed to id/object/created/owned_by,
// and an Anthropic-shaped one that carries context windows (max_input_tokens).
// CLIProxyAPI is the worked example — its OpenAI handler filters to four fields
// on purpose, while its Anthropic handler returns the whole registry object.
//
// Sent only as a fallback, never by default: providers are not obliged to
// understand it, and ping.go deliberately avoids it for that reason.
const (
	anthropicVersionHeader = "Anthropic-Version"
	anthropicVersionValue  = "2023-06-01"
)

// FetchModels tries to get the current model list from a provider.
// Uses modelsURL if set, otherwise falls back to baseURL.
// Strategy: try modelsURL/v1/models first, then strip path prefix and try
// root-level /v1/models as fallback.
//
// If an endpoint answers but reports no context window for any model, it is
// asked once more for its Anthropic-shaped listing, which some gateways only
// return when that header is present.
func FetchModels(baseURL, modelsURL, apiKey string) ([]ModelInfo, error) {
	searchURL := modelsURL
	if searchURL == "" {
		searchURL = baseURL
	}

	for _, candidate := range modelListURLs(searchURL) {
		models, err := fetch(candidate, apiKey, nil)
		if err != nil {
			continue
		}
		if reportsAnyWindow(models) {
			return models, nil
		}

		// The endpoint answered, but told us nothing about context windows. Ask
		// for the Anthropic-shaped listing before giving up. One extra request,
		// and only for providers that reported no window at all. A failed or
		// still-windowless retry is discarded in favour of the response that
		// actually worked.
		rich, retryErr := fetch(candidate, apiKey, map[string]string{
			anthropicVersionHeader: anthropicVersionValue,
		})
		if retryErr == nil && reportsAnyWindow(rich) {
			return rich, nil
		}
		return models, nil
	}
	return nil, fmt.Errorf("could not fetch models from any endpoint")
}

// reportsAnyWindow reports whether at least one model carries a context window.
func reportsAnyWindow(models []ModelInfo) bool {
	for _, m := range models {
		if m.ContextWindow() > 0 {
			return true
		}
	}
	return false
}

func modelListURLs(baseURL string) []string {
	cleaned := strings.TrimRight(baseURL, "/")
	var candidates []string

	// If the URL already points at a models endpoint (e.g. the user pasted a
	// full ".../compatible-mode/v1/models"), try it verbatim first.
	if strings.HasSuffix(cleaned, "/models") {
		candidates = append(candidates, cleaned)
	}

	candidates = append(candidates, cleaned+"/v1/models")

	if u, err := url.Parse(cleaned); err == nil && u.Path != "" && u.Path != "/" {
		u.Path = "/v1/models"
		candidates = append(candidates, u.String())
	}
	return candidates
}

func fetch(url, apiKey string, extraHeaders map[string]string) ([]ModelInfo, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
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
	return decodeModels(limited)
}

// decodeModels reads the {"data": [...]} envelope that both the OpenAI-style
// and Anthropic-style model listings use. Split out from fetch so the field
// shapes can be tested without a network round-trip.
func decodeModels(r io.Reader) ([]ModelInfo, error) {
	var result struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(r).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse models: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("provider returned no models")
	}

	return result.Data, nil
}
