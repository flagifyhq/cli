package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/ui"
)

// deviceSlowDownStep is the interval increase RFC 8628 §3.5 requires on each
// slow_down response.
const deviceSlowDownStep = 5 * time.Second

var (
	errDeviceExpired = errors.New("Code expired. Run 'flagify auth login --device' again.")
	errDeviceDenied  = errors.New("Authorization was denied in the console.")
)

// deviceWait blocks for d or until ctx is done. Seam so tests can observe the
// polling cadence without sleeping.
var deviceWait = func(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// exitProcess is os.Exit, seamed so the Ctrl-C path is testable.
var exitProcess = os.Exit

type deviceTokenPoller interface {
	PollDeviceToken(deviceCode string) (*api.DeviceTokenResult, error)
}

// loginDevice runs the OAuth 2.0 Device Authorization Grant (RFC 8628): it
// prints a one-time code and URL the user approves from any other device,
// then polls until the console decision arrives. Credentials are written only
// on success; Ctrl-C exits 130 without touching the config.
func loginDevice(cfg *config.Config) error {
	if cfg.DeviceID == "" {
		return fmt.Errorf("profile session identity is missing")
	}
	hostname, _ := os.Hostname()

	client := api.NewClient("")
	if cfg.APIUrl != "" {
		client.SetBaseURL(cfg.APIUrl)
	}

	codeResp, err := client.RequestDeviceCode(cfg.DeviceID, hostname)
	if err != nil {
		return fmt.Errorf("could not start device login: %w", err)
	}

	var out io.Writer = os.Stdout
	if !ui.IsTTY() {
		out = os.Stderr
	}
	fmt.Fprintln(out, ui.Warning(fmt.Sprintf("First copy your one-time code: %s", codeResp.UserCode)))
	fmt.Fprintln(out, fmt.Sprintf("%s Open %s on any device", ui.Arrow(), codeResp.VerificationURI))
	fmt.Fprintln(out, fmt.Sprintf("  (or: %s)", ui.Dim(codeResp.VerificationURIComplete)))
	fmt.Fprintln(out, fmt.Sprintf("%s Waiting for authorization… (expires in %s, Ctrl-C to cancel)", ui.Arrow(), humanizeMinutes(codeResp.ExpiresIn)))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(codeResp.ExpiresIn)*time.Second)
	defer cancel()

	stopInterruptWatch := watchInterrupt(out)
	result, err := pollDeviceToken(ctx, client, codeResp.DeviceCode, time.Duration(codeResp.Interval)*time.Second)
	stopInterruptWatch()
	if err != nil {
		return err
	}

	cfg.AccessToken = result.Tokens.AccessToken
	cfg.RefreshToken = result.Tokens.RefreshToken
	cfg.Token = ""
	cfg.Workspace = ""
	cfg.WorkspaceID = ""
	cfg.Project = ""
	cfg.ProjectID = ""
	cfg.Environment = ""

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	email, _ := result.User["email"].(string)
	device := hostname
	if device == "" {
		device = cfg.DeviceID
	}
	fmt.Println(ui.Success(fmt.Sprintf("Authenticated as %s on device %s", email, device)))
	maybeAutoSelect(cfg)
	return nil
}

// watchInterrupt exits the process with status 130 on Ctrl-C while the device
// flow is waiting. The returned func restores default signal handling; it is
// called before credentials are saved so an interrupt can never land midway
// through our own config write.
func watchInterrupt(out io.Writer) func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	done := make(chan struct{})
	go func() {
		select {
		case <-signals:
			fmt.Fprintln(out)
			exitProcess(130)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(signals)
		close(done)
	}
}

// pollDeviceToken is the single continuous RFC 8628 polling loop. It waits
// `interval` before each poll, keeps waiting on authorization_pending, adds
// deviceSlowDownStep on slow_down, and stops on success, a terminal protocol
// error, any other error, or when ctx (bounded by expires_in) is done.
func pollDeviceToken(ctx context.Context, poller deviceTokenPoller, deviceCode string, interval time.Duration) (*api.DeviceTokenResult, error) {
	for {
		if err := deviceWait(ctx, interval); err != nil {
			return nil, errDeviceExpired
		}

		result, err := poller.PollDeviceToken(deviceCode)
		switch {
		case err == nil:
			return result, nil
		case errors.Is(err, api.ErrAuthorizationPending):
			continue
		case errors.Is(err, api.ErrSlowDown):
			interval += deviceSlowDownStep
		case errors.Is(err, api.ErrExpiredToken):
			return nil, errDeviceExpired
		case errors.Is(err, api.ErrAccessDenied):
			return nil, errDeviceDenied
		default:
			return nil, fmt.Errorf("device login failed: %w", err)
		}
	}
}

// humanizeMinutes formats an expires_in value (seconds) as "N min".
func humanizeMinutes(seconds int) string {
	return fmt.Sprintf("%d min", seconds/60)
}
