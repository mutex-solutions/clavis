package provider

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Some gateways serve a trimmed model list unless asked for the Anthropic shape.
// CLIProxyAPI answers /v1/models with only id/object/created/owned_by by default
// (a deliberate filter in its OpenAI handler), and returns the full object —
// including max_input_tokens — when Anthropic-Version is present. These tests
// pin the fallback that makes the documented "use the window the provider
// reports" path work against those gateways.

func TestFetchModels_RetriesWithAnthropicHeaderWhenNoWindowReported(t *testing.T) {
	var bareCalls, anthropicCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(anthropicVersionHeader) == "" {
			bareCalls++
			io.WriteString(w, `{"data":[{"id":"glm","object":"model","owned_by":"gateway"}]}`)
			return
		}
		anthropicCalls++
		if got := r.Header.Get(anthropicVersionHeader); got != anthropicVersionValue {
			t.Errorf("Anthropic-Version = %q, want %q", got, anthropicVersionValue)
		}
		io.WriteString(w, `{"data":[{"id":"glm","object":"model","max_input_tokens":1048576}]}`)
	}))
	defer srv.Close()

	models, err := FetchModels(srv.URL, "", "k")
	if err != nil {
		t.Fatalf("FetchModels() error: %v", err)
	}
	if bareCalls != 1 || anthropicCalls != 1 {
		t.Errorf("requests: bare=%d anthropic=%d, want 1 and 1", bareCalls, anthropicCalls)
	}
	if got := ContextWindowFor(models, "glm"); got != 1048576 {
		t.Errorf("context window = %d, want 1048576", got)
	}
}

func TestFetchModels_NoSecondRequestWhenWindowAlreadyReported(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		io.WriteString(w, `{"data":[{"id":"m","context_length":200000}]}`)
	}))
	defer srv.Close()

	models, err := FetchModels(srv.URL, "", "k")
	if err != nil {
		t.Fatalf("FetchModels() error: %v", err)
	}
	if calls != 1 {
		t.Errorf("made %d requests, want 1 — the fallback must not fire when a window is present", calls)
	}
	if got := ContextWindowFor(models, "m"); got != 200000 {
		t.Errorf("context window = %d, want 200000", got)
	}
}

func TestFetchModels_FailedFallbackKeepsWorkingResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(anthropicVersionHeader) != "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		io.WriteString(w, `{"data":[{"id":"m","owned_by":"gateway"}]}`)
	}))
	defer srv.Close()

	// A provider that errors on the extra header — or simply has no window to
	// report — must still yield its models. Reporting no window is the state
	// this project already handled before the fallback existed.
	models, err := FetchModels(srv.URL, "", "k")
	if err != nil {
		t.Fatalf("a failed fallback must not discard the working response: %v", err)
	}
	if len(models) != 1 || models[0].ID != "m" {
		t.Errorf("got %+v, want the single model from the first response", models)
	}
}

func TestFetchModels_StillWindowlessAfterRetryKeepsOriginal(t *testing.T) {
	// Both shapes answer with no window: a gateway that has none to give, or a
	// provider that returns the same body regardless of headers. The retry is
	// then wasted work, and the original list is what we return.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":[{"id":"m","owned_by":"gateway"}]}`)
	}))
	defer srv.Close()

	models, err := FetchModels(srv.URL, "", "k")
	if err != nil {
		t.Fatalf("FetchModels() error: %v", err)
	}
	if got := ContextWindowFor(models, "m"); got != 0 {
		t.Errorf("context window = %d, want 0", got)
	}
}
