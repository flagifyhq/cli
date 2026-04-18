package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flagifyhq/cli/internal/api"
)

func newTestServerWithFlags(t *testing.T, flags []api.Flag) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(flags)
	}))
}

func TestFindFlagEnvIDSuccess(t *testing.T) {
	flags := []api.Flag{{
		Key: "dev-tools",
		Environments: []api.FlagEnv{
			{ID: "fe-dev", EnvironmentKey: "development"},
			{ID: "fe-prod", EnvironmentKey: "production"},
		},
	}}
	srv := newTestServerWithFlags(t, flags)
	defer srv.Close()

	client := api.NewClient("token")
	client.SetBaseURL(srv.URL)

	id, err := findFlagEnvID(client, "proj", "dev-tools", "production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "fe-prod" {
		t.Fatalf("got %q, want fe-prod", id)
	}
}

func TestFindFlagEnvIDFlagNotFound(t *testing.T) {
	srv := newTestServerWithFlags(t, []api.Flag{{Key: "other-flag"}})
	defer srv.Close()

	client := api.NewClient("token")
	client.SetBaseURL(srv.URL)

	_, err := findFlagEnvID(client, "proj", "dev-tools", "development")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrFlagNotFound) {
		t.Fatalf("error does not wrap ErrFlagNotFound: %v", err)
	}
}

func TestFindFlagEnvIDEnvNotFound(t *testing.T) {
	flags := []api.Flag{{
		Key: "dev-tools",
		Environments: []api.FlagEnv{
			{ID: "fe-dev", EnvironmentKey: "development"},
		},
	}}
	srv := newTestServerWithFlags(t, flags)
	defer srv.Close()

	client := api.NewClient("token")
	client.SetBaseURL(srv.URL)

	_, err := findFlagEnvID(client, "proj", "dev-tools", "production")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("error does not wrap ErrEnvNotFound: %v", err)
	}
}
