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
	ID           string          `json:"id"`
	DisplayName  string          `json:"displayName"`
	BaseURL      string          `json:"baseUrl"`
	Tier         string          `json:"tier"`
	Auth         AuthConfig      `json:"auth"`
	Models       []ProviderModel `json:"models"`
	LastVerified string          `json:"lastVerified"`
}

// AuthConfig describes how to authenticate with a provider.
type AuthConfig struct {
	NeedsKey bool   `json:"needsKey"`
	KeyURL   string `json:"keyUrl"`
	EnvVar   string `json:"envVar"`
}

// ProviderModel is a model available from a provider.
type ProviderModel struct {
	ID        string   `json:"id"`
	Free      bool     `json:"free"`
	Context   int      `json:"context"`
	Note      string   `json:"note"`
	TierHints []string `json:"tierHints"`
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

// All returns the full built-in provider catalog.
func All() []Provider {
	return catalog
}

// Get returns a provider by ID.
func Get(id string) (*Provider, error) {
	for i := range catalog {
		if catalog[i].ID == id {
			return &catalog[i], nil
		}
	}
	return nil, fmt.Errorf("provider %q not found", id)
}

// ModelsForTier returns models whose tierHints include the given tier.
func (p *Provider) ModelsForTier(tier string) []ProviderModel {
	var out []ProviderModel
	for _, m := range p.Models {
		for _, hint := range m.TierHints {
			if hint == tier {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// DefaultForTier returns the first model recommended for a tier, or empty string.
func (p *Provider) DefaultForTier(tier string) string {
	models := p.ModelsForTier(tier)
	if len(models) > 0 {
		return models[0].ID
	}
	return ""
}
