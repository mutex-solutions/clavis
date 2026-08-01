package provider

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPingSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"type":"message","content":[{"type":"text","text":"hi"}]}`))
	}))
	defer srv.Close()

	if err := Ping(srv.URL, "test-key", "test-model"); err != nil {
		t.Errorf("Ping() = %v, want nil", err)
	}
}

func TestPingBadKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	defer srv.Close()

	if err := Ping(srv.URL, "bad-key", "test-model"); err == nil {
		t.Error("Ping() with bad key should error")
	}
}

func TestPingRefused(t *testing.T) {
	// Bind a port then immediately close it — connection will be refused instantly.
	// No timeout wait like with unreachable hosts.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind: %v", err)
	}
	addr := l.Addr().String()
	l.Close()

	err = Ping("http://"+addr, "test-key", "test-model")
	if err == nil {
		t.Error("Ping() should fail for refused connection")
	}
}

func TestPingInvalidURL(t *testing.T) {
	if err := Ping("://invalid", "test-key", "test-model"); err == nil {
		t.Error("Ping() should fail for invalid URL")
	}
}

func TestPingNonJSONError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`Internal Server Error`))
	}))
	defer srv.Close()

	if err := Ping(srv.URL, "test-key", "test-model"); err == nil {
		t.Error("Ping() should error on 500")
	}
}

func TestPingTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	if err := Ping(srv.URL+"/", "test-key", "test-model"); err != nil {
		t.Errorf("Ping() with trailing slash: %v", err)
	}
}
