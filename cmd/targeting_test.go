package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flagifyhq/cli/internal/api"
)

// The CLI no longer resolves (project, flag key, env key) into a
// flag_environments ULID locally — it hits /v1/projects/{pid}/flags/{key}/...
// directly and lets the API resolve. These tests verify the URL the client
// produces and that the API's 404 messages reach the caller.

func TestGetTargetingRulesByKey_URL(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]api.TargetingRule{})
	}))
	defer srv.Close()

	client := api.NewClient("token")
	client.SetBaseURL(srv.URL)

	if _, err := client.GetTargetingRulesByKey("proj_01", "checkout-redesign", "production"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/v1/projects/proj_01/flags/checkout-redesign/environments/production/targeting-rules"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestSetTargetingRulesByKey_URL(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]api.TargetingRule{})
	}))
	defer srv.Close()

	client := api.NewClient("token")
	client.SetBaseURL(srv.URL)

	if _, err := client.SetTargetingRulesByKey("proj_01", "checkout-redesign", "prod-eu", map[string]any{"rules": []any{}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/v1/projects/proj_01/flags/checkout-redesign/environments/prod-eu/targeting-rules"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestGetTargetingRulesByKey_404PropagatesAPIMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found","message":"environment \"qa-2\" not found in project"}`))
	}))
	defer srv.Close()

	client := api.NewClient("token")
	client.SetBaseURL(srv.URL)

	_, err := client.GetTargetingRulesByKey("proj_01", "checkout-redesign", "qa-2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "qa-2") {
		t.Fatalf("error %q should mention the unknown env slug", err)
	}
}
