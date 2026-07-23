package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/flagifyhq/cli/internal/ui"
	"github.com/flagifyhq/cli/internal/updater"
	"github.com/spf13/cobra"
)

// User-facing messages for `flagify update`. Kept in one block so the wording
// stays consistent and testable.
const (
	msgChecking            = "Checking for updates..."
	msgUpToDate            = "You already have the latest version %s"
	msgNewVersionAvailable = "A new version is available: %s"
	msgConfirmInstall      = "Install version %s?"
	msgDownloading         = "Downloading and installing %s..."
	msgUpdated             = "Successfully updated to %s"
	msgDevBuild            = "Running a development build (%s) — skipping version check. Install a released build to use 'flagify update'."
	msgManagedByFmt        = "Installed via %s. Run this to update:\n\n  %s\n"
	msgNoAssetForPlatform  = "No release asset available for %s/%s — download manually from https://github.com/flagifyhq/cli/releases"
	msgNonTTYNeedsYes      = "flagify update requires --yes when stdout is not a terminal (CI/scripts) — this action replaces the CLI binary and cannot be undone automatically"
	msgPermissionDenied    = "Permission denied writing to %s — retry with sudo, or download the binary manually from https://github.com/flagifyhq/cli/releases"
	msgChecksumMismatch    = "Downloaded asset failed checksum verification — aborting without touching the current binary. Please retry, or download manually from https://github.com/flagifyhq/cli/releases"
	msgWindowsFileInUse    = "Could not replace the running binary — it may be in use by another 'flagify' process. Close it and retry, or download manually from https://github.com/flagifyhq/cli/releases"
	msgRateLimited         = "GitHub API rate limit reached (60 requests/hour, unauthenticated). Try again later, or download manually from https://github.com/flagifyhq/cli/releases"
	msgReleaseNotFound     = "No release found on GitHub — this may be a temporary issue, try again shortly"
	msgMalformedRelease    = "GitHub returned an unexpected release format — aborting. This may be a temporary GitHub issue, try again shortly"
	msgNetworkError        = "Could not reach GitHub to check for updates: %v"
)

// Test seams — overridable so update_test.go can force TTY/non-TTY, synthetic
// executable paths, and stub the install without replacing the test binary.
// Mirrors the confirmProjectFileClear pattern in context.go.
var (
	isInteractive   = ui.IsTTY
	resolveExecPath = defaultResolveExecPath
	installRelease  = updater.InstallRelease
)

// defaultResolveExecPath returns the real, symlink-resolved path of the running
// binary.
func defaultResolveExecPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		// Fall back to the unresolved path rather than failing the update.
		return exe, nil
	}
	return resolved, nil
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the flagify CLI to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		checkOnly, _ := cmd.Flags().GetBool("check")
		force, _ := cmd.Flags().GetBool("force")
		yes, _ := cmd.Flags().GetBool("yes")

		// A dev build has no numeric version to compare — short-circuit before
		// any network call.
		if Version == "dev" || Version == "" {
			fmt.Println(ui.Warning(fmt.Sprintf(msgDevBuild, Version)))
			return nil
		}

		fmt.Println(ui.Info(msgChecking))

		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()

		release, err := updater.LatestRelease(ctx, nil)
		if err != nil {
			return mapCheckError(err)
		}

		cmp, err := updater.CompareVersions(Version, release.TagName)
		if err != nil {
			return errors.New(msgMalformedRelease)
		}

		if cmp >= 0 && !force {
			fmt.Println(ui.Success(fmt.Sprintf(msgUpToDate, ui.Cyan(Version))))
			return nil
		}

		fmt.Println(ui.Info(fmt.Sprintf(msgNewVersionAvailable, ui.Cyan(release.TagName))))

		// --check never installs and never requires --yes or a TTY.
		if checkOnly {
			return nil
		}

		resolvedPath, err := resolveExecPath()
		if err != nil {
			return fmt.Errorf("could not resolve the running binary path: %w", err)
		}

		method := updater.DetectInstallMethod(resolvedPath)
		if method != updater.MethodDirect {
			fmt.Println(ui.Info(fmt.Sprintf(msgManagedByFmt, method.Label(), ui.Bold(method.UpgradeCommand()))))
			return nil
		}

		// Direct install: apply the confirmation gate. Unlike the rest of the
		// CLI, a non-TTY install without --yes is refused because replacing the
		// running binary has no automatic rollback.
		if !isInteractive() {
			if !yes {
				return errors.New(msgNonTTYNeedsYes)
			}
		} else {
			confirmed, err := ui.Confirm(fmt.Sprintf(msgConfirmInstall, release.TagName), yes)
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Println(ui.Info("Cancelled."))
				return nil
			}
		}

		fmt.Println(ui.Info(fmt.Sprintf(msgDownloading, release.TagName)))

		if err := installRelease(ctx, release, resolvedPath, isInteractive()); err != nil {
			return mapInstallError(err, resolvedPath)
		}

		fmt.Println(ui.Success(fmt.Sprintf(msgUpdated, ui.Cyan(release.TagName))))
		return nil
	},
}

// mapCheckError translates a LatestRelease failure into its actionable message.
func mapCheckError(err error) error {
	switch {
	case errors.Is(err, updater.ErrRateLimited):
		return errors.New(msgRateLimited)
	case errors.Is(err, updater.ErrReleaseNotFound):
		return errors.New(msgReleaseNotFound)
	case errors.Is(err, updater.ErrMalformedRelease):
		return errors.New(msgMalformedRelease)
	default:
		return fmt.Errorf(msgNetworkError, err)
	}
}

// mapInstallError translates an InstallRelease failure into its actionable
// message.
func mapInstallError(err error, targetPath string) error {
	switch {
	case errors.Is(err, updater.ErrChecksumMismatch):
		return errors.New(msgChecksumMismatch)
	case errors.Is(err, updater.ErrNoAssetForPlatform):
		return fmt.Errorf(msgNoAssetForPlatform, runtime.GOOS, runtime.GOARCH)
	case errors.Is(err, updater.ErrWindowsFileInUse):
		return errors.New(msgWindowsFileInUse)
	case os.IsPermission(err):
		return fmt.Errorf(msgPermissionDenied, filepath.Dir(targetPath))
	default:
		return err
	}
}

func init() {
	updateCmd.Flags().Bool("check", false, "Check for a new version without installing it")
	updateCmd.Flags().Bool("force", false, "Reinstall even if already on the latest version")
	rootCmd.AddCommand(updateCmd)
}
