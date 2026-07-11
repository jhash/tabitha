package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPurgeURLsSendsBearerTokenAndFileList(t *testing.T) {
	var gotAuth string
	var gotBody purgeRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	c := &Client{APIToken: "test-token", ZoneID: "zone123", HTTPClient: server.Client(), baseURL: server.URL}
	err := c.PurgeURLs(context.Background(), []string{"https://tabitha.jakehash.com/"})
	if err != nil {
		t.Fatalf("PurgeURLs() error = %v", err)
	}

	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
	}
	if len(gotBody.Files) != 1 || gotBody.Files[0] != "https://tabitha.jakehash.com/" {
		t.Errorf("purge request files = %v, want [https://tabitha.jakehash.com/]", gotBody.Files)
	}
}

func TestPurgeURLsReturnsErrorOnAPIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"success": false, "errors": [{"code": 10000, "message": "Authentication error"}]}`))
	}))
	defer server.Close()

	c := &Client{APIToken: "bad-token", ZoneID: "zone123", HTTPClient: server.Client(), baseURL: server.URL}
	err := c.PurgeURLs(context.Background(), []string{"https://tabitha.jakehash.com/"})
	if err == nil {
		t.Fatal("PurgeURLs() error = nil, want an error on API failure")
	}
}

func TestPurgeURLsNoopsOnEmptyList(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	c := &Client{APIToken: "t", ZoneID: "z", HTTPClient: server.Client(), baseURL: server.URL}
	if err := c.PurgeURLs(context.Background(), nil); err != nil {
		t.Fatalf("PurgeURLs() error = %v", err)
	}
	if called {
		t.Error("expected no API call for an empty URL list")
	}
}
