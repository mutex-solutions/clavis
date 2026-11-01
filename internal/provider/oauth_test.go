package provider

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckOAuthSendsBearerAndVersion(t *testing.T) {
	var gotAuth, gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("anthropic-version")
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, `{"data":[]}`)
	}))
	defer srv.Close()

	if err := CheckOAuth(srv.URL, "sk-ant-oat-token"); err != nil {
		t.Fatalf("CheckOAuth: %v", err)
	}
	if gotAuth != "Bearer sk-ant-oat-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotVersion != anthropicVersion {
		t.Errorf("anthropic-version = %q, want %q", gotVersion, anthropicVersion)
	}
}

func TestCheckOAuthRejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"invalid token"}}`)
	}))
	defer srv.Close()

	err := CheckOAuth(srv.URL, "bad")
	if err == nil {
		t.Fatal("CheckOAuth accepted a 401")
	}
}

func TestFetchModelsOAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		io.WriteString(w, `{"data":[{"id":"claude-sonnet-5-5"},{"id":"claude-opus-5-5"}]}`)
	}))
	defer srv.Close()

	models, err := FetchModelsOAuth(srv.URL, "", "tok")
	if err != nil {
		t.Fatalf("FetchModelsOAuth: %v", err)
	}
	if len(models) != 2 || models[0].ID != "claude-sonnet-5-5" {
		t.Errorf("models = %+v", models)
	}
}
