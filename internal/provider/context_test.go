package provider

import (
	"strings"
	"testing"
)

// TestDecodeModelsContextWindows covers the three shapes a provider can report
// a context window in, since guessing wrong here silently mis-sizes every
// auto-compaction decision Claude Code makes.
func TestDecodeModelsContextWindows(t *testing.T) {
	cases := []struct {
		name string
		body string
		id   string
		want int
	}{
		{
			// OpenRouter: top-level context_length, plus a per-endpoint one.
			// The endpoint actually serving the request is the narrower truth.
			name: "openrouter top_provider wins over context_length",
			body: `{"data":[{"id":"deepseek/deepseek-chat","object":"model",
			        "context_length":131072,
			        "top_provider":{"context_length":65536}}]}`,
			id:   "deepseek/deepseek-chat",
			want: 65536,
		},
		{
			name: "openrouter context_length alone",
			body: `{"data":[{"id":"qwen/qwen3","context_length":131072}]}`,
			id:   "qwen/qwen3",
			want: 131072,
		},
		{
			name: "anthropic-style max_input_tokens",
			body: `{"data":[{"id":"claude-sonnet-5","max_input_tokens":200000}]}`,
			id:   "claude-sonnet-5",
			want: 200000,
		},
		{
			// The bare-ID case: the provider lists IDs and nothing else, which
			// must read as "unknown" and not as zero.
			name: "no window reported",
			body: `{"data":[{"id":"deepseek-flash","object":"model"}]}`,
			id:   "deepseek-flash",
			want: 0,
		},
		{
			name: "explicit zero is not a window",
			body: `{"data":[{"id":"m","context_length":0,"max_input_tokens":0}]}`,
			id:   "m",
			want: 0,
		},
		{
			name: "a zero endpoint limit falls through to the family maximum",
			body: `{"data":[{"id":"m","context_length":128000,"top_provider":{"context_length":0}}]}`,
			id:   "m",
			want: 128000,
		},
		{
			name: "empty top_provider object is harmless",
			body: `{"data":[{"id":"m","context_length":128000,"top_provider":{}}]}`,
			id:   "m",
			want: 128000,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			models, err := decodeModels(strings.NewReader(c.body))
			if err != nil {
				t.Fatalf("decodeModels() error: %v", err)
			}
			if got := ContextWindowFor(models, c.id); got != c.want {
				t.Errorf("ContextWindowFor(%q) = %d, want %d", c.id, got, c.want)
			}
		})
	}
}

func TestDecodeModelsRejectsEmptyList(t *testing.T) {
	// An endpoint that answers 200 with no models must be treated as a failed
	// lookup, so FetchModels falls through to the next candidate URL instead of
	// handing the wizard an empty picker.
	if _, err := decodeModels(strings.NewReader(`{"data":[]}`)); err == nil {
		t.Fatal("decodeModels() of an empty list should error")
	}
	if _, err := decodeModels(strings.NewReader(`not json`)); err == nil {
		t.Fatal("decodeModels() of garbage should error")
	}
}

func TestContextWindowForUnknownModel(t *testing.T) {
	models, err := decodeModels(strings.NewReader(
		`{"data":[{"id":"a","context_length":1000}]}`))
	if err != nil {
		t.Fatalf("decodeModels() error: %v", err)
	}
	if got := ContextWindowFor(models, "missing"); got != 0 {
		t.Errorf("ContextWindowFor(missing) = %d, want 0", got)
	}
	if got := ContextWindowFor(nil, "a"); got != 0 {
		t.Errorf("ContextWindowFor(nil models) = %d, want 0", got)
	}
}
