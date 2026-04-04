package cmd

import (
	"bufio"
	"context"
	"fmt"
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
	"golang.org/x/term"
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
	defaultConsoleURL      = "https://console.flagify.dev"
	localConsoleURL        = "https://local-console.flagify.dev"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Flagify",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()

		if cfg.IsLoggedIn() {
			fmt.Println(ui.Info("Already logged in. Use 'flagify logout' to sign out first."))
			return nil
		}

		classic, _ := cmd.Flags().GetBool("classic")
		if classic {
			return loginClassic(cfg)
		}

		return loginBrowser(cfg)
	},
}

func loginBrowser(cfg *config.Config) error {
	hostname, _ := os.Hostname()
	deviceID := "cli-" + hostname

	consoleURL := cfg.ConsoleUrl
	if consoleURL == "" {
		if strings.Contains(cfg.APIUrl, "localhost") || strings.HasPrefix(cfg.APIUrl, "http://local-") {
			consoleURL = localConsoleURL
		} else {
			consoleURL = defaultConsoleURL
		}
	}

	// Start local callback server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return fmt.Errorf("failed to start local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	type callbackResult struct {
		accessToken  string
		refreshToken string
		err          error
	}
	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		accessToken := r.URL.Query().Get("access_token")
		refreshToken := r.URL.Query().Get("refresh_token")

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if accessToken == "" || refreshToken == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, callbackHTML("Authentication Failed", "Missing tokens. Please try again.", true))
			resultCh <- callbackResult{err: fmt.Errorf("missing tokens in callback")}
			return
		}

		fmt.Fprint(w, callbackHTML("Authentication Successful", "You can close this tab and return to your terminal.", false))
		resultCh <- callbackResult{accessToken: accessToken, refreshToken: refreshToken}
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	// Open browser
	authURL := fmt.Sprintf("%s/auth/cli-auth?p=%d&did=%s", consoleURL, port, url.QueryEscape(deviceID))

	fmt.Printf("%s Opening browser to authenticate...\n", ui.Arrow())
	fmt.Printf("  %s\n\n", ui.Dim(authURL))

	if err := browser.OpenURL(authURL); err != nil {
		fmt.Printf("%s Could not open browser. Please visit the URL above manually.\n", ui.Warning(""))
	}

	fmt.Printf("%s Waiting for authorization...\n", ui.Arrow())

	// Wait for callback or timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	select {
	case result := <-resultCh:
		server.Shutdown(context.Background())
		if result.err != nil {
			return fmt.Errorf("authentication failed: %w", result.err)
		}

		cfg.AccessToken = result.accessToken
		cfg.RefreshToken = result.refreshToken
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

	case <-ctx.Done():
		server.Shutdown(context.Background())
		return fmt.Errorf("authentication timed out. Try again or use 'flagify login --classic'")
	}
}

func loginClassic(cfg *config.Config) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s %s ", ui.Arrow(), ui.Bold("Email:"))
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Printf("%s %s ", ui.Arrow(), ui.Bold("Password:"))
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println()
	password := string(passwordBytes)

	client := api.NewClient("")
	if cfg.APIUrl != "" {
		client.SetBaseURL(cfg.APIUrl)
	}

	hostname, _ := os.Hostname()
	deviceID := "cli-" + hostname

	result, err := client.Login(email, password, deviceID)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	cfg.AccessToken = result.Tokens.AccessToken
	cfg.RefreshToken = result.Tokens.RefreshToken
	cfg.Token = ""
	cfg.Workspace = ""
	cfg.Project = ""
	cfg.Environment = ""

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	name := ""
	if n, ok := result.User["name"].(string); ok {
		name = n
	}
	if name != "" {
		fmt.Println(ui.Success(fmt.Sprintf("Logged in as %s %s", ui.Bold(name), ui.Dim("("+email+")"))))
	} else {
		fmt.Println(ui.Success(fmt.Sprintf("Logged in as %s", ui.Bold(email))))
	}
	maybeAutoSelect(cfg)
	return nil
}

func callbackHTML(title, message string, isError bool) string {
	color := "#00CC88"
	icon := "✓"
	if isError {
		color = "#FF6B6B"
		icon = "✗"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Flagify CLI</title></head>
<body style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0;background:#0A0E17;color:#F8FAFC;">
  <div style="text-align:center;max-width:400px;padding:2rem;">
    <div style="font-size:3rem;margin-bottom:1rem;color:%s;">%s</div>
    <h1 style="font-size:1.25rem;margin-bottom:0.5rem;">%s</h1>
    <p style="color:#94A3B8;font-size:0.9rem;">%s</p>
  </div>
</body>
</html>`, color, icon, title, message)
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out of Flagify",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		cfg.AccessToken = ""
		cfg.RefreshToken = ""
		cfg.Token = ""

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println(ui.Success("Logged out."))
		return nil
	},
}

func init() {
	loginCmd.Flags().Bool("classic", false, "Use email/password authentication")
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}
