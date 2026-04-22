package config

import (
	"errors"
	"os"
)

// Source tags where a resolved field came from. Surfaced in `flagify status`
// so users can see exactly which signal won.
type Source string

const (
	SourceFlag        Source = "flag"
	SourceEnv         Source = "env"
	SourceProjectFile Source = "project-file"
	SourceBinding     Source = "binding"
	SourceProfile     Source = "profile-default"
	SourceDefault     Source = "default"
)

// Built-in fallbacks for the API/console URL. Only applied when no profile,
// project file, env, or flag provides a value.
const (
	DefaultAPIUrl     = "https://api.flagify.dev"
	DefaultConsoleUrl = "https://console.flagify.dev"
)

// ErrAmbiguousProfile is returned when the store has multiple accounts and
// the resolver has no way (flag, env, binding, preferredProfile, or current)
// to pick one. The CLI converts this into an actionable error message.
var ErrAmbiguousProfile = errors.New("ambiguous profile: multiple accounts, no selection signal")

// FlagValues is the subset of CLI flag values the resolver cares about.
// Extracted by the caller from cobra — keeps this package cobra-free.
type FlagValues struct {
	Profile     string
	Workspace   string
	WorkspaceID string
	Project     string
	ProjectID   string
	Environment string
	APIUrl      string
}

// EnvValues is the subset of FLAGIFY_* env vars the resolver cares about.
type EnvValues struct {
	Profile      string
	Workspace    string
	WorkspaceID  string
	Project      string
	ProjectID    string
	Environment  string
	APIUrl       string
	AccessToken  string
	RefreshToken string
}

// EnvFromOS snapshots the current process environment into an EnvValues.
// Callers running inside tests should build EnvValues explicitly instead.
func EnvFromOS() EnvValues {
	return EnvValues{
		Profile:      os.Getenv("FLAGIFY_PROFILE"),
		Workspace:    os.Getenv("FLAGIFY_WORKSPACE"),
		WorkspaceID:  os.Getenv("FLAGIFY_WORKSPACE_ID"),
		Project:      os.Getenv("FLAGIFY_PROJECT"),
		ProjectID:    os.Getenv("FLAGIFY_PROJECT_ID"),
		Environment:  os.Getenv("FLAGIFY_ENVIRONMENT"),
		APIUrl:       os.Getenv("FLAGIFY_API_URL"),
		AccessToken:  os.Getenv("FLAGIFY_ACCESS_TOKEN"),
		RefreshToken: os.Getenv("FLAGIFY_REFRESH_TOKEN"),
	}
}

// ResolveInput bundles everything Resolve needs so the function stays pure.
type ResolveInput struct {
	Flags FlagValues
	Env   EnvValues
	Store *Store
	CWD   string // starting directory for the project-file walk
}

// ResolvedConfig is the final, field-by-field resolved view used by commands.
// Sources maps field names (`profile`, `workspace`, `workspaceId`, `project`,
// `projectId`, `environment`, `apiUrl`, `consoleUrl`) to the winning Source.
type ResolvedConfig struct {
	Profile     string
	Account     *Account
	Workspace   string
	WorkspaceID string
	Project     string
	ProjectID   string
	Environment string
	APIUrl      string
	ConsoleUrl  string
	ProjectFile *ProjectFile

	// EnvAccessToken / EnvRefreshToken carry FLAGIFY_ACCESS_TOKEN and
	// FLAGIFY_REFRESH_TOKEN when they were set. When non-empty, clients must
	// use these for the request and MUST NOT persist refreshed tokens to the
	// store — the user opted in to an ephemeral identity for this run only.
	EnvAccessToken  string
	EnvRefreshToken string

	Sources map[string]Source
}

// ProjectIdentifier returns the best project handle to send to the API: the
// ULID if present, otherwise the slug. Empty when neither resolved.
func (r *ResolvedConfig) ProjectIdentifier() string {
	if r == nil {
		return ""
	}
	if r.ProjectID != "" {
		return r.ProjectID
	}
	return r.Project
}

// WorkspaceIdentifier returns the best workspace handle to send to the API.
func (r *ResolvedConfig) WorkspaceIdentifier() string {
	if r == nil {
		return ""
	}
	if r.WorkspaceID != "" {
		return r.WorkspaceID
	}
	return r.Workspace
}

// HasToken reports whether the resolved account has an access token usable by
// the API client. Env-provided tokens short-circuit the check.
func (r *ResolvedConfig) HasToken() bool {
	if r == nil {
		return false
	}
	if r.Account != nil && r.Account.AccessToken != "" {
		return true
	}
	return false
}

// Resolve applies the precedence rules defined in
// `Flagify Docs/decisions/2026-04-22-cli-multi-account.md` and returns a
// ResolvedConfig plus the Sources map. It never mutates Store, Env, or Flags.
//
// Precedence per field: flag > env > project file > binding > profile default > built-in.
// Within the same level, IDs win over slugs (the caller decides which to consume
// when both are available — this function just surfaces both).
func Resolve(input ResolveInput) (*ResolvedConfig, error) {
	rc := &ResolvedConfig{Sources: map[string]Source{}}

	// 1. Locate project file (needed for profile + field resolution).
	if input.CWD != "" {
		pf, err := FindProjectFile(input.CWD)
		if err != nil {
			return nil, err
		}
		rc.ProjectFile = pf
	}

	// 2. Resolve which profile owns this invocation.
	profile, profileSrc, err := resolveProfile(input, rc.ProjectFile)
	if err != nil {
		return nil, err
	}
	rc.Profile = profile
	if profile != "" {
		rc.Sources["profile"] = profileSrc
		if input.Store != nil {
			rc.Account = input.Store.Accounts[profile]
		}
	}

	// 3. Resolve each scope field independently.
	rc.Workspace, rc.Sources["workspace"] = resolveField(
		input.Flags.Workspace,
		input.Env.Workspace,
		projectFileValue(rc.ProjectFile, func(d ProjectFileData) string { return d.Workspace }),
		accountDefault(rc.Account, func(d Defaults) string { return d.Workspace }),
	)
	rc.WorkspaceID, rc.Sources["workspaceId"] = resolveField(
		input.Flags.WorkspaceID,
		input.Env.WorkspaceID,
		projectFileValue(rc.ProjectFile, func(d ProjectFileData) string { return d.WorkspaceID }),
		accountDefault(rc.Account, func(d Defaults) string { return d.WorkspaceID }),
	)
	rc.Project, rc.Sources["project"] = resolveField(
		input.Flags.Project,
		input.Env.Project,
		projectFileValue(rc.ProjectFile, func(d ProjectFileData) string { return d.Project }),
		accountDefault(rc.Account, func(d Defaults) string { return d.Project }),
	)
	rc.ProjectID, rc.Sources["projectId"] = resolveField(
		input.Flags.ProjectID,
		input.Env.ProjectID,
		projectFileValue(rc.ProjectFile, func(d ProjectFileData) string { return d.ProjectID }),
		accountDefault(rc.Account, func(d Defaults) string { return d.ProjectID }),
	)
	rc.Environment, rc.Sources["environment"] = resolveField(
		input.Flags.Environment,
		input.Env.Environment,
		projectFileValue(rc.ProjectFile, func(d ProjectFileData) string { return d.Environment }),
		accountDefault(rc.Account, func(d Defaults) string { return d.Environment }),
	)

	// 4. URLs — project file has no say here, but env and profile do.
	apiUrl, apiSrc := resolveAPIUrl(input, rc.Account)
	rc.APIUrl = apiUrl
	rc.Sources["apiUrl"] = apiSrc

	consoleUrl, consoleSrc := resolveConsoleUrl(rc.Account)
	rc.ConsoleUrl = consoleUrl
	rc.Sources["consoleUrl"] = consoleSrc

	// Ephemeral token overrides (env only — never written back).
	rc.EnvAccessToken = input.Env.AccessToken
	rc.EnvRefreshToken = input.Env.RefreshToken

	// Strip sources for fields that ended up empty so callers can test presence
	// without the map being littered with zero entries.
	for k, v := range rc.Sources {
		if v == "" {
			delete(rc.Sources, k)
		}
	}

	return rc, nil
}

// resolveProfile implements the 6-step profile resolution defined in the
// decision log: flag → env → binding → preferredProfile → current → single-account.
func resolveProfile(input ResolveInput, pf *ProjectFile) (string, Source, error) {
	if input.Flags.Profile != "" {
		return input.Flags.Profile, SourceFlag, nil
	}
	if input.Env.Profile != "" {
		return input.Env.Profile, SourceEnv, nil
	}

	s := input.Store
	if s == nil {
		return "", "", nil
	}

	// Binding for this repo, if a project file exists.
	if pf != nil {
		if b, ok := BindingFor(s, pf.Dir); ok && b.Profile != "" {
			if _, exists := s.Accounts[b.Profile]; exists {
				return b.Profile, SourceBinding, nil
			}
			// Ghost binding — fall through silently.
		}
	}

	// preferredProfile hint, only if the profile actually exists locally.
	if pf != nil && pf.Data.PreferredProfile != "" {
		if _, exists := s.Accounts[pf.Data.PreferredProfile]; exists {
			return pf.Data.PreferredProfile, SourceProjectFile, nil
		}
	}

	// Current profile.
	if s.Current != "" {
		if _, exists := s.Accounts[s.Current]; exists {
			return s.Current, SourceProfile, nil
		}
	}

	// Exactly one account → use it unambiguously.
	if len(s.Accounts) == 1 {
		for name := range s.Accounts {
			return name, SourceProfile, nil
		}
	}

	// Multiple accounts and no signal → ambiguous.
	if len(s.Accounts) > 1 {
		return "", "", ErrAmbiguousProfile
	}

	return "", "", nil
}

func resolveField(flag, env, projectFile, profileDefault string) (string, Source) {
	if flag != "" {
		return flag, SourceFlag
	}
	if env != "" {
		return env, SourceEnv
	}
	if projectFile != "" {
		return projectFile, SourceProjectFile
	}
	if profileDefault != "" {
		return profileDefault, SourceProfile
	}
	return "", ""
}

func resolveAPIUrl(input ResolveInput, acc *Account) (string, Source) {
	if input.Flags.APIUrl != "" {
		return input.Flags.APIUrl, SourceFlag
	}
	if input.Env.APIUrl != "" {
		return input.Env.APIUrl, SourceEnv
	}
	if acc != nil && acc.APIUrl != "" {
		return acc.APIUrl, SourceProfile
	}
	return DefaultAPIUrl, SourceDefault
}

func resolveConsoleUrl(acc *Account) (string, Source) {
	if acc != nil && acc.ConsoleUrl != "" {
		return acc.ConsoleUrl, SourceProfile
	}
	return DefaultConsoleUrl, SourceDefault
}

func projectFileValue(pf *ProjectFile, get func(ProjectFileData) string) string {
	if pf == nil {
		return ""
	}
	return get(pf.Data)
}

func accountDefault(acc *Account, get func(Defaults) string) string {
	if acc == nil {
		return ""
	}
	return get(acc.Defaults)
}
