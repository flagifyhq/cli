package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// resetWebhookFlags clears flag state on the package-level command tree
// between tests. Without it, a flag value set in one test leaks into the
// next and breaks the "missing flag" assertions.
func resetWebhookFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		webhooksCreateCmd.Flags().Set("name", "")
		webhooksCreateCmd.Flags().Set("url", "")
		webhooksCreateCmd.Flags().Set("events", "")
		webhooksDeleteCmd.Flags().Set("yes", "false")
	})
}

func TestWebhooksList_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/webhooks") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	writeTestConfig(t, srv.URL)

	out, err := runRoot(t, "webhooks", "list", "-p", "proj_01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No webhooks found") {
		t.Fatalf("expected empty-state message, got: %q", out)
	}
}

func TestWebhooksList_Renders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":        "wh_01",
				"projectId": "proj_01",
				"name":      "Slack",
				"url":       "https://hooks.slack.com/services/x",
				"events":    []string{"flag.toggled"},
				"active":    true,
				"createdAt": "2026-04-27T00:00:00Z",
				"updatedAt": "2026-04-27T00:00:00Z",
			},
		})
	}))
	defer srv.Close()
	writeTestConfig(t, srv.URL)

	out, err := runRoot(t, "webhooks", "list", "-p", "proj_01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"Slack", "hooks.slack.com", "flag.toggled", "active"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestWebhooksCreate_RequiresName(t *testing.T) {
	resetWebhookFlags(t)
	writeTestConfig(t, "http://127.0.0.1:0")
	_, err := runRoot(t, "webhooks", "create", "-p", "proj_01", "--url", "https://x.example.com")
	if err == nil || !strings.Contains(err.Error(), "--name is required") {
		t.Fatalf("expected --name error, got: %v", err)
	}
}

func TestWebhooksCreate_RequiresURL(t *testing.T) {
	resetWebhookFlags(t)
	writeTestConfig(t, "http://127.0.0.1:0")
	_, err := runRoot(t, "webhooks", "create", "-p", "proj_01", "--name", "Test")
	if err == nil || !strings.Contains(err.Error(), "--url is required") {
		t.Fatalf("expected --url error, got: %v", err)
	}
}

func TestWebhooksCreate_PrintsSecretOnce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/webhooks") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":        "wh_01",
			"projectId": "proj_01",
			"name":      "Slack",
			"url":       "https://hooks.slack.com/services/x",
			"events":    []string{"flag.created", "flag.toggled"},
			"active":    true,
			"secret":    "whsec_abc123",
			"createdAt": "2026-04-27T00:00:00Z",
			"updatedAt": "2026-04-27T00:00:00Z",
		})
	}))
	defer srv.Close()
	resetWebhookFlags(t)
	writeTestConfig(t, srv.URL)

	out, err := runRoot(t, "webhooks", "create",
		"-p", "proj_01",
		"--name", "Slack",
		"--url", "https://hooks.slack.com/services/x",
		"--events", "flag.created,flag.toggled",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"whsec_abc123", "won't be shown again", "Created webhook"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}
