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
	Auth        AuthConfig `json:"auth"`
}

// AuthConfig describes how to authenticate with a provider.
type AuthConfig struct {
	NeedsKey bool   `json:"needsKey"`
	KeyURL   string `json:"keyUrl"`
	EnvVar   string `json:"envVar"`
}

// ModelInfo is returned by the /v1/models endpoint.
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
	Created int64  `json:"created,omitempty"`
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
