package provider

import (
	"strings"
	"testing"
)

func TestModelListURLs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "plain base url",
			in:   "https://api.deepseek.com/anthropic",
			want: []string{
				"https://api.deepseek.com/anthropic/v1/models",
				"https://api.deepseek.com/v1/models",
			},
		},
		{
			name: "root base url",
			in:   "https://api.example.com",
			want: []string{"https://api.example.com/v1/models"},
		},
		{
			name: "trailing slash trimmed",
			in:   "https://api.example.com/anthropic/",
			want: []string{
				"https://api.example.com/anthropic/v1/models",
				"https://api.example.com/v1/models",
			},
		},
		{
			// The Alibaba MaaS case: a full models URL is tried verbatim first.
			name: "full models url used verbatim first",
			in:   "https://host.com/compatible-mode/v1/models",
			want: []string{
				"https://host.com/compatible-mode/v1/models",
				"https://host.com/compatible-mode/v1/models/v1/models",
				"https://host.com/v1/models",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := modelListURLs(c.in)
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("modelListURLs(%q)\n got: %v\nwant: %v", c.in, got, c.want)
			}
		})
	}
}

func TestModelListURLs_VerbatimModelsFirst(t *testing.T) {
	// A URL already ending in /models must be attempted before any derived
	// candidate, so a user-provided endpoint is honored exactly.
	got := modelListURLs("https://x.com/compatible-mode/v1/models")
	if got[0] != "https://x.com/compatible-mode/v1/models" {
		t.Errorf("first candidate = %q, want the verbatim models URL", got[0])
	}
}
