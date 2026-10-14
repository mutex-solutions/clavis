package provider

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed providers.json
var catalogFS embed.FS

// Provider is a known Anthropic-compatible API endpoint.
type Provider struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"displayName"`
	BaseURL     string     `json:"baseUrl"`
	ModelsURL   string     `json:"modelsUrl,omitempty"` // optional, falls back to baseUrl
	Auth        AuthConfig `json:"auth"`
}

// AuthConfig describes how to authenticate with a provider.
type AuthConfig struct {
	NeedsKey bool   `json:"needsKey"`
	KeyURL   string `json:"keyUrl"`
	EnvVar   string `json:"envVar"`
}

// ModelInfo is returned by the /v1/models endpoint.
//
// The context-window fields are best-effort: gateways that report a window use
// different spellings, and many report nothing at all. See ContextWindow.
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
	Created int64  `json:"created,omitempty"`

	// ContextLength is the OpenAI-compatible spelling (OpenRouter and the
	// gateways built on it).
	ContextLength *int `json:"context_length,omitempty"`
	// MaxInputTokens is the Anthropic spelling.
	MaxInputTokens *int `json:"max_input_tokens,omitempty"`
	// TopProvider carries per-endpoint metadata on OpenRouter, whose
	// context_length is the model family's maximum rather than the limit of the
	// endpoint actually serving it.
	TopProvider *TopProvider `json:"top_provider,omitempty"`
}

// TopProvider is OpenRouter's per-endpoint block.
type TopProvider struct {
	ContextLength *int `json:"context_length,omitempty"`
}

// ContextWindow returns the model's context window in tokens, or 0 when the
// provider didn't report one. The narrowest reliable value wins: an endpoint's
// own limit (top_provider) beats the model family's advertised maximum, which
// beats Anthropic's max_input_tokens.
func (m ModelInfo) ContextWindow() int {
	for _, n := range []*int{
		topProviderContext(m.TopProvider),
		m.ContextLength,
		m.MaxInputTokens,
	} {
		if n != nil && *n > 0 {
			return *n
		}
	}
	return 0
}

func topProviderContext(tp *TopProvider) *int {
	if tp == nil {
		return nil
	}
	return tp.ContextLength
}

// ContextWindowFor returns the reported context window for a model ID, or 0 if
// the model isn't in the list or carries no window.
func ContextWindowFor(models []ModelInfo, id string) int {
	for _, m := range models {
		if m.ID == id {
			return m.ContextWindow()
		}
	}
	return 0
}

var catalog []Provider

func init() {
	data, err := catalogFS.ReadFile("providers.json")
	if err != nil {
		panic(fmt.Sprintf("clavis: failed to read embedded provider catalog: %v", err))
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		panic(fmt.Sprintf("clavis: failed to parse provider catalog: %v", err))
	}
}

// All returns a copy of the built-in provider catalog.
func All() []Provider {
	result := make([]Provider, len(catalog))
	copy(result, catalog)
	return result
}

// Get returns a copy of a provider by ID.
func Get(id string) (Provider, error) {
	for i := range catalog {
		if catalog[i].ID == id {
			return catalog[i], nil
		}
	}
	return Provider{}, fmt.Errorf("provider %q not found", id)
}

// ModelsEndpoint returns the models endpoint URL for this provider.
// Uses ModelsURL if set, otherwise uses BaseURL.
func (p *Provider) ModelsEndpoint() string {
	if p.ModelsURL != "" {
		return p.ModelsURL
	}
	return p.BaseURL
}
