# Test Suite Audit — `flagifyhq/cli`

> Reviewed: 2026-03-30  
> Auditor role: Senior Software Engineer & Test Auditor  
> Scope: All test files in the repository

---

## Files Audited

| File | Tests |
|------|-------|
| `internal/api/client_test.go` | 10 |
| `internal/config/config_test.go` | 6 |
| `internal/ui/ui_test.go` | 5 |
| `cmd/ai_setup_test.go` | 7 |

---

## `internal/api/client_test.go`

### `TestClientNoRefreshOnAuthEndpoints` — Wrong test name, wrong behavior tested

- **Problem:** The test name implies it verifies that calls to `/v1/auth/*` endpoints bypass the refresh loop. In reality it only tests that a missing `refreshToken` prevents a refresh attempt. No auth-path URL is involved at all — the test server serves a generic `/v1/test` endpoint.
- **Why dangerous:** A future change that accidentally triggers refresh on auth endpoints (infinite loop or double-login) would not be caught by this test, despite the name suggesting it covers that scenario.
- **Suggested fix:** Rename to `TestClientNoRefreshWithoutRefreshToken`. Add a separate `TestClientNoRefreshOnAuthEndpoints` that calls `/v1/auth/login` with a refresh token set and confirms no refresh is attempted.

---

### `TestClientAutoRefreshOn401` — Testify assertions in server goroutine

- **Problem:** `assert.Equal(t, "******", r.Header.Get("Authorization"))` lives inside the `httptest` server handler, which runs in a separate goroutine. Calling `t.Fail()` from a goroutine other than the test goroutine is explicitly warned against in the Go testing docs and can trigger races.
- **Why dangerous:** On failure the assertion may be silently swallowed or cause a panic.
- **Suggested fix:** Capture the header in a local variable inside the handler and assert on it after `client.Get()` returns:
  ```go
  var secondCallAuth string
  // in handler: secondCallAuth = r.Header.Get("Authorization")
  // after client.Get: assert.Equal(t, "Bearer new-access", secondCallAuth)
  ```

---

### `TestClientRefreshFailsFallsBack` — Coincidental assertion

- **Problem:** Both the original 401 and the refresh 401 return the same code (`"unauthorized"`). The assertion `Contains(t, err.Error(), "unauthorized")` would pass whether the code returns the original error or the refresh error — the two are indistinguishable.
- **Why dangerous:** If the implementation erroneously returns the *refresh* error instead of the original, the test still passes. The spec says "return original error" but this is not validated.
- **Suggested fix:** Give the original 401 a distinct message (e.g. `"original-error"`) and the refresh 401 a different one (e.g. `"refresh-failed"`), then assert the original message is present and the refresh message is absent.

---

### `TestClientRevokeKeys` — No verification of request or response

- **Problem:** Only asserts `require.NoError(t, err)`. The HTTP method (`POST`) and path (`/v1/environments/env1/keys/revoke`) are never verified.
- **Why dangerous:** If the implementation calls `GET` instead of `POST`, or uses the wrong path, this test still passes.
- **Suggested fix:** Add `assert.Equal(t, "POST", r.Method)` and `assert.Equal(t, "/v1/environments/env1/keys/revoke", r.URL.Path)` inside the handler, mirroring `TestClientListKeys`.

---

### `TestClientLogin` — Does not verify request body

- **Problem:** The server handler does not decode or assert on the login request body (`email`, `password`, `deviceId`). Only the response is checked.
- **Why dangerous:** The client could send empty or wrong credentials and the test would pass.
- **Suggested fix:** Decode `r.Body` inside the handler and assert all three fields are correctly sent.

---

### Missing coverage — `Patch`, `Put`, `Delete`, `Register`, `ListFlags`, `CreateFlag`, `GetFlag`, `ToggleFlag`

- **Problem:** Eight methods on the API client have zero test coverage. This includes the primary feature-flag operations (`ListFlags`, `CreateFlag`, `ToggleFlag`) that are the core of the product.
- **Why dangerous:** Silent regressions in flag operations (wrong path, wrong method, wrong body) would go completely undetected.
- **Suggested fix:** Add one test per method following the `TestClientListWorkspaces` pattern.

---

### Missing coverage — Network-level failure scenarios

- **Problem:** No test exercises what happens when the HTTP call itself fails (connection refused, timeout, DNS failure). The client uses a 10-second timeout but this is never tested.
- **Suggested fix:** Use `httptest.NewServer`, close it *before* calling the client, or use a deliberately invalid URL, to simulate connection errors.

---

## `internal/config/config_test.go`

### `TestConfigSaveAndLoad` and others — `os.Setenv("HOME", ...)` race condition

- **Problem:** `os.Setenv` modifies the process-wide environment. If tests ever run in parallel (`t.Parallel()`), multiple tests modifying `$HOME` concurrently will corrupt each other's state. The `defer os.Unsetenv("HOME")` also does not restore the *original* value — it removes the variable entirely.
- **Why dangerous:** Currently passes in serial mode but is a silent time bomb; adding `t.Parallel()` anywhere would cause data races and flaky failures.
- **Suggested fix:** Use `t.Setenv("HOME", tmpDir)` (Go 1.17+), which automatically restores the original value and is parallel-safe.

---

### `TestConfigLoadMissing` — Incomplete empty-state assertion

- **Problem:** Only checks `assert.Empty(t, cfg.AccessToken)`. Other fields (`RefreshToken`, `Project`, `Workspace`, etc.) are not verified.
- **Why dangerous:** If `Load()` were changed to return a non-zero struct, the test would still pass.
- **Suggested fix:** Compare the entire loaded config to `&config.Config{}` or assert all fields individually.

---

### `TestGetToken` — Missing zero-value case

- **Problem:** Does not test the case where both `AccessToken` and `Token` are empty. The function should return `""` but this path is untested.
- **Suggested fix:** Add `cfg = &config.Config{}; assert.Equal(t, "", cfg.GetToken())`.

---

### `TestPath` — Weak path assertion

- **Problem:** `assert.Contains(t, path, ".flagify")` and `assert.Contains(t, path, "config.json")` would pass for a wrong path like `/tmp/.flagify-backup/config.json.bak`.
- **Suggested fix:** Use exact equality: `assert.Equal(t, filepath.Join(tmpDir, ".flagify", "config.json"), path)`.

---

### `TestConfigJSONCamelCase` — Unchecked errors in setup

- **Problem:** `os.MkdirAll(dir, 0o700)` and `os.WriteFile(...)` return errors that are silently ignored. If the write fails, the test silently asserts on an empty config.
- **Suggested fix:** Wrap with `require.NoError(t, err)`.

---

### Missing coverage — Malformed JSON / unreadable config file

- **Problem:** No test for: (a) a config file that exists but contains invalid JSON, (b) a file that cannot be read due to permissions.
- **Suggested fix:** Write a malformed JSON file and assert `config.Load()` returns a non-nil error.

---

## `internal/ui/ui_test.go`

### `TestSuccess`, `TestInfo`, `TestWarning` — Weak `Contains` assertions

- **Problem:** All three tests only check that the message and icon appear *somewhere* in the output. They do not verify ordering, ANSI wrapping, or that the message is not doubled.
- **Why dangerous:** If `Success` returned `"done ✓"` (icon after message), tests still pass. Styling could be stripped entirely.
- **Suggested fix:** Use `assert.HasPrefix` or a regex asserting the icon appears before the message. Also test empty-string and special-character inputs.

---

### `TestTableWithRows` — Missing header assertion

- **Problem:** Does not assert that `"Name"` (the second header) appears in the output.
- **Suggested fix:** Add `assert.Contains(t, result, "Name")`.

---

### `TestTableEmpty` — No negative assertion for row content

- **Problem:** Checks headers appear but does not assert the output contains no data rows.
- **Suggested fix:** Assert the rendered string does not contain unexpected row content.

---

### `TestKeyValue` — Padding behavior not tested

- **Problem:** The implementation uses `fmt.Sprintf("%-12s", label)` for alignment. This padding is never verified.
- **Suggested fix:** Pass a short label and verify the expected padded form in the output.

---

### `ui.Error()` has no test at all.

---

## `cmd/ai_setup_test.go`

### `TestGenerateClaude` — Weak path assertion

- **Problem:** `assert.Contains(t, path, "CLAUDE.md")` passes for any path containing that substring, including wrong paths.
- **Suggested fix:** `assert.Equal(t, "CLAUDE.md + .claude/commands/", path)`.

---

### `TestGenerateClaudeAppendsToExisting` — Replace branch untested

- **Problem:** `generateClaude` has three code paths: (1) no file, (2) file without `## Feature Flags` — append, (3) file *with* `## Feature Flags` — replace. Only path 2 is tested. The splice/replace logic is complex and error-prone.
- **Why dangerous:** A bug in the replace path (dropped trailing content, duplicate sections) would not be caught.
- **Suggested fix:** Add `TestGenerateClaudeReplacesExistingFlagsSection` that pre-writes a `CLAUDE.md` with a `## Feature Flags` block plus trailing content, calls `generateClaude`, and asserts the old section is replaced, new section is present, and trailing content is preserved.

---

### `TestGenerateClaudeAppendsToExisting` — No duplicate-content guard

- **Problem:** Only checks `Contains("My Project")` and `Contains("Feature Flags")`. If content was accidentally written twice, both assertions still pass.
- **Suggested fix:** Assert `strings.Count(content, "## Feature Flags") == 1`.

---

### `TestGenerateCursor`, `TestGenerateWindsurf`, `TestGenerateCopilot` — Minimal content assertions

- **Problem:** Checked strings (e.g. `"useFlag"`, `"isEnabled"`) appear in multiple templates. If templates were swapped, most assertions would still pass.
- **Suggested fix:** Assert unique, tool-specific strings per template (e.g. `.cursorrules` literal for Cursor, `.windsurfrules` literal for Windsurf).

---

### `TestGenerateClaude` — Slash command content not verified

- **Problem:** The test only asserts the slash command files *exist* (`os.ReadFile` returns no error) but never inspects their contents.
- **Suggested fix:** Assert key strings in each slash command file.

---

### Missing coverage — `fetchFlagsContext` has zero tests

- **Problem:** The function that fetches live flags, builds the markdown table, and handles "not logged in" / "no project" error paths is entirely untested.
- **Suggested fix:** Test with a mock HTTP server returning flags, an empty flag list, and error responses.

---

### Missing coverage — `generateToolConfig` unknown-tool error path

- **Problem:** The `default:` branch returning an error is never exercised.
- **Suggested fix:** Call `generateToolConfig("vscode", data)` and assert an error is returned.

---

### Missing coverage — `isValidTool` case sensitivity

- **Problem:** `isValidTool("Claude")`, `isValidTool("CURSOR")` etc. are not tested. The implementation is case-sensitive (correct) but this is unverified.
- **Suggested fix:** Add assertions for title-cased and all-caps variants.

---

## Final Summary

### Overall Test Suite Reliability Score: **3 / 10**

The tests cover the happy path for a small subset of functionality. Large portions of the codebase (all flag operations, all Cobra command handlers, `fetchFlagsContext`, several HTTP methods) have no test coverage at all. Existing tests suffer from coincidental passing, misleading names, and weak string-contains assertions that cannot detect logic inversions or content duplication.

---

### Top 5 Risks

| # | Risk |
|---|------|
| 1 | **Core feature operations are untested.** `ListFlags`, `CreateFlag`, `ToggleFlag`, `GetFlag`, `Patch`, `Put`, `Delete`, and `Register` have no tests. This is the main product functionality. |
| 2 | **Misleading test name creates false confidence.** `TestClientNoRefreshOnAuthEndpoints` does not test what its name says, leaving a gap in the token-refresh loop invariant. |
| 3 | **The CLAUDE.md replace-section logic is complex and entirely untested.** The string-splice code in `generateClaude` is high-risk (off-by-one errors, dropped trailing content) with zero coverage. |
| 4 | **Global environment mutation is test-order sensitive.** `os.Setenv("HOME", ...)` without restoring the original value means test isolation depends on execution order. |
| 5 | **No negative/error-path tests for I/O.** Malformed JSON config, unreadable files, missing templates, and network-level failures are all untested. |

---

### Concrete Recommendations

1. **Add API method coverage** — Write one test per public method on `*Client` using the existing `httptest` pattern. Verify method, path, request body, and response parsing for each.

2. **Fix `TestClientNoRefreshOnAuthEndpoints`** — Rename and split into two tests: one for the "no refresh token" case and a new one for the "auth-path skips refresh" invariant.

3. **Add `TestGenerateClaudeReplacesExistingFlagsSection`** — Cover the third branch in `generateClaude` and verify trailing content is preserved and no duplication occurs.

4. **Use `t.Setenv` instead of `os.Setenv`** — `t.Setenv(key, value)` (Go 1.17+) automatically restores the original value and is parallel-safe. Replace all `os.Setenv` calls in tests.

5. **Add command-level integration tests** — Use Cobra's `ExecuteC()` with a mock API client (via dependency injection or an interface) to test the full command invocation flow, including error messages and exit codes.
