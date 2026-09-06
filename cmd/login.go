package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/picker"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

// maybeAutoSelect auto-selects workspace (if only one) and then picks a project.
func maybeAutoSelect(cfg *config.Config) {
	client := api.NewClient(cfg.AccessToken)
	if cfg.APIUrl != "" {
		client.SetBaseURL(cfg.APIUrl)
	}

	workspaces, err := client.ListWorkspaces()
	if err != nil || len(workspaces) != 1 {
		return
	}

	ws := workspaces[0]
	cfg.Workspace = ws.Slug
	cfg.WorkspaceID = ws.ID
	if err := config.Save(cfg); err != nil {
		return
	}
	fmt.Println(ui.Info(fmt.Sprintf("Workspace set to %s %s", ui.Bold(ws.Name), ui.Dim("("+ws.Slug+")"))))

	project, err := picker.PickProject(client, ws.ID)
	if err != nil {
		return
	}

	cfg.Project = project.Slug
	cfg.ProjectID = project.ID
	if err := config.Save(cfg); err != nil {
		return
	}
	fmt.Println(ui.Success(fmt.Sprintf("Project set to %s %s", ui.Bold(project.Name), ui.Dim("("+project.Slug+")"))))
	fmt.Println(ui.Dim("To change project, run: flagify projects pick"))
}

const (
	defaultConsoleURL = "https://console.flagify.dev"
	localConsoleURL   = "https://local-console.flagify.dev"
)

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Flagify (add or refresh a profile)",
	RunE:  runLogin,
}

// runLogin materializes the target profile as current before entering the
// browser flow. This way existing helpers (loginBrowser, maybeAutoSelect) can
// keep using config.Load/Save via the shim — the shim always writes to the
// active profile, which is the one we just selected.
func runLogin(cmd *cobra.Command, args []string) error {
	requestedProfile, _ := cmd.Flags().GetString("profile")
	profile, cfg, err := prepareLoginProfile(requestedProfile, rand.Reader)
	if err != nil {
		return err
	}

	if err := loginBrowser(cfg); err != nil {
		return err
	}

	maybeAdoptProfileForRepo(profile)
	return nil
}

func prepareLoginProfile(profile string, random io.Reader) (string, *config.Config, error) {
	store, err := config.LoadOrMigrate()
	if err != nil {
		return "", nil, err
	}

	if profile == "" {
		profile = store.Current
	}
	if profile == "" {
		profile = config.DefaultProfile
	}

	if _, ok := store.Accounts[profile]; !ok {
		store.Accounts[profile] = &config.Account{}
	}
	if err := ensureProfileDeviceID(store.Accounts[profile], random); err != nil {
		return "", nil, fmt.Errorf("failed to create profile session identity: %w", err)
	}
	store.Current = profile
	if err := config.SaveStore(store); err != nil {
		return "", nil, err
	}

	cfg, err := config.Load()
	if err != nil {
		return "", nil, err
	}
	return profile, cfg, nil
}

const profileDeviceIDBytes = 16

// ensureProfileDeviceID assigns one stable, opaque session identity to a local
// profile. The value is deliberately unrelated to the hostname and profile
// name so two profiles for the same user map to independent backend sessions.
func ensureProfileDeviceID(account *config.Account, random io.Reader) error {
	if account == nil {
		return fmt.Errorf("nil profile")
	}
	if account.DeviceID != "" {
		return nil
	}
	bytes := make([]byte, profileDeviceIDBytes)
	if _, err := io.ReadFull(random, bytes); err != nil {
		return err
	}
	account.DeviceID = "cli-" + hex.EncodeToString(bytes)
	return nil
}

// adoptCandidate inspects the repo at cwd and returns the project file that
// should be offered a preferredProfile rewrite for the freshly-logged-in
// profile. Returns nil when no rewrite is warranted: no profile, no project
// file in the walk, or the pin already matches. Pure function — separated
// from the prompt so it can be covered in tests without a TTY.
func adoptCandidate(cwd, profile string) *config.ProjectFile {
	if profile == "" || cwd == "" {
		return nil
	}
	pf, err := config.FindProjectFile(cwd)
	if err != nil || pf == nil {
		return nil
	}
	if pf.Data.PreferredProfile == profile {
		return nil
	}
	return pf
}

// maybeAdoptProfileForRepo offers to rewrite the repo's .flagify/project.json
// preferredProfile to the profile the user just logged into. Skipped outside
// a TTY — non-interactive shells (CI, scripts) must never silently rewrite a
// committable file on the user's behalf.
func maybeAdoptProfileForRepo(profile string) {
	if !ui.IsTTY() {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	pf := adoptCandidate(cwd, profile)
	if pf == nil {
		return
	}

	previous := pf.Data.PreferredProfile
	if previous == "" {
		previous = "(none)"
	}
	fmt.Println(ui.Info(fmt.Sprintf(
		"This repo pins preferredProfile=%s in %s.",
		ui.Bold(previous),
		displayPath(pf.Path),
	)))
	confirmed, err := ui.Confirm(
		fmt.Sprintf("Set preferredProfile to %q for this repo?", profile),
		false,
	)
	if err != nil || !confirmed {
		fmt.Println(ui.Dim(fmt.Sprintf(
			"Keeping preferredProfile=%s. Run 'flagify project set preferred-profile %s' to change it later.",
			previous, profile,
		)))
		return
	}

	pf.Data.PreferredProfile = profile
	if _, err := config.WriteProjectFile(pf.Dir, pf.Data); err != nil {
		fmt.Println(ui.Warning("Failed to update project file: " + err.Error()))
		return
	}
	fmt.Println(ui.Success(fmt.Sprintf(
		"Updated preferredProfile to %s in %s",
		ui.Bold(profile),
		displayPath(pf.Path),
	)))
}

// maxLoginAttempts bounds the browser OAuth flow: the initial attempt plus two
// reopens. Each reopen recovers from a callback that arrived without tokens
// (expired browser session or an interrupted/stale flow). Non-TTY contexts cap
// this at 1 — reopening a browser in CI/scripts is meaningless.
const maxLoginAttempts = 3

// errAuthTimedOut and errAuthIncomplete are the terminal outcomes of the login
// loop. errAuthIncomplete is the single actionable message shown when every
// attempt's callback lacked tokens (and the only message a non-TTY run gets).
var (
	errAuthTimedOut   = errors.New("authentication timed out")
	errAuthIncomplete = errors.New("could not complete authentication — the browser session may have expired or the flow was interrupted. Run `flagify auth login` again")
)

type attemptStatus int

const (
	attemptSuccess attemptStatus = iota
	attemptExpired               // callback arrived without tokens
	attemptTimeout               // outer context deadline hit
	attemptError                 // setup failure (e.g. listener could not bind)
)

type attemptOutcome struct {
	status       attemptStatus
	accessToken  string
	refreshToken string
	err          error
}

type callbackResult struct {
	accessToken  string
	refreshToken string
	err          error
}

func resolveConsoleURL(cfg *config.Config) string {
	if cfg.ConsoleUrl != "" {
		return cfg.ConsoleUrl
	}
	if strings.Contains(cfg.APIUrl, "localhost") || strings.HasPrefix(cfg.APIUrl, "http://local-") {
		return localConsoleURL
	}
	return defaultConsoleURL
}

// runLoginLoop drives the bounded retry over an injectable per-attempt function
// so the decision logic is testable without a real browser or listener. It
// retries only on attemptExpired, up to `attempts`; success returns tokens,
// timeout and setup errors return immediately, and an exhausted budget returns
// the single actionable errAuthIncomplete. It never writes credentials — the
// caller persists only on a clean success, so failures leave no partial state.
func runLoginLoop(attempts int, attempt func(attemptNo int) attemptOutcome) (accessToken, refreshToken string, err error) {
	for i := 1; i <= attempts; i++ {
		outcome := attempt(i)
		switch outcome.status {
		case attemptSuccess:
			return outcome.accessToken, outcome.refreshToken, nil
		case attemptTimeout:
			return "", "", errAuthTimedOut
		case attemptError:
			return "", "", outcome.err
		case attemptExpired:
			if i < attempts {
				fmt.Println(ui.Warning(fmt.Sprintf(
					"Authorization did not complete (no tokens received). Retrying… (attempt %d of %d)",
					i+1, attempts,
				)))
				continue
			}
		}
	}
	return "", "", errAuthIncomplete
}

// runBrowserAttempt performs one OAuth attempt: bind a fresh listener, open the
// browser to the console, and wait on the callback or the shared outer context.
// The server is shut down on return (defer), so each attempt gets a fresh
// listener/port/channel and no stale result leaks into a later attempt.
func runBrowserAttempt(ctx context.Context, consoleURL, deviceID string) attemptOutcome {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return attemptOutcome{status: attemptError, err: fmt.Errorf("failed to start local server: %w", err)}
	}
	port := listener.Addr().(*net.TCPAddr).Port

	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		accessToken := r.URL.Query().Get("access_token")
		refreshToken := r.URL.Query().Get("refresh_token")

		if accessToken == "" || refreshToken == "" {
			redirectURL := fmt.Sprintf("%s/auth/cli-auth?status=error", consoleURL)
			http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
			resultCh <- callbackResult{err: fmt.Errorf("missing tokens in callback")}
			return
		}

		redirectURL := fmt.Sprintf("%s/auth/cli-auth?status=success", consoleURL)
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		resultCh <- callbackResult{accessToken: accessToken, refreshToken: refreshToken}
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	authURL := fmt.Sprintf("%s/auth/cli-auth?p=%d&did=%s", consoleURL, port, url.QueryEscape(deviceID))

	fmt.Printf("%s Opening browser to authenticate...\n", ui.Arrow())
	fmt.Printf("  %s\n\n", ui.Dim(authURL))

	if err := browser.OpenURL(authURL); err != nil {
		fmt.Printf("%s Could not open browser. Please visit the URL above manually.\n", ui.Warning(""))
	}

	fmt.Printf("%s Waiting for authorization...\n", ui.Arrow())

	select {
	case result := <-resultCh:
		if result.err != nil {
			return attemptOutcome{status: attemptExpired}
		}
		return attemptOutcome{status: attemptSuccess, accessToken: result.accessToken, refreshToken: result.refreshToken}
	case <-ctx.Done():
		return attemptOutcome{status: attemptTimeout}
	}
}

func loginBrowser(cfg *config.Config) error {
	if cfg.DeviceID == "" {
		return fmt.Errorf("profile session identity is missing")
	}
	consoleURL := resolveConsoleURL(cfg)

	// One outer timeout bounds the whole flow across all attempts.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	attempts := maxLoginAttempts
	if !ui.IsTTY() {
		attempts = 1 // never reopen the browser in CI/scripts
	}

	accessToken, refreshToken, err := runLoginLoop(attempts, func(int) attemptOutcome {
		return runBrowserAttempt(ctx, consoleURL, cfg.DeviceID)
	})
	if err != nil {
		return err
	}

	cfg.AccessToken = accessToken
	cfg.RefreshToken = refreshToken
	cfg.Token = ""
	cfg.Workspace = ""
	cfg.WorkspaceID = ""
	cfg.Project = ""
	cfg.ProjectID = ""
	cfg.Environment = ""

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Println(ui.Success("Authenticated successfully."))
	maybeAutoSelect(cfg)
	return nil
}

func init() {
	authLoginCmd.Flags().String("profile", "", "Profile to create or update (defaults to current, or 'default')")

	authCmd.AddCommand(authLoginCmd)
}
