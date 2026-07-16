# Charm OIDC Integration & Local Verification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the bingo charm's `charmcraft.yaml` to Charmed Hydra's `oauth` Juju relation, adapt `internal/config` to consume the env vars that relation produces, and document a manual runbook for verifying the full OIDC login flow against a locally-deployed Canonical Identity Platform.

**Architecture:** Add an optional `oauth` relation (interface `oauth`, matching `charms.hydra.v0.oauth`'s vendored library) plus three `paas_charm`-recognised config options to `charm/charmcraft.yaml`. `paas_charm.go.Charm` handles the relation lifecycle and injects `APP_OAUTH_*` / `APP_SECRET_KEY` env vars automatically — no `charm.py` changes needed. `internal/config.Load()` gains a fallback chain so each OIDC field resolves from the plain `OIDC_*`/`SESSION_SECRET` env vars first, then from the charm-provided `APP_OAUTH_*`/`APP_SECRET_KEY` vars, deriving the redirect URL from `APP_BASE_URL` + the app's fixed `/auth/callback` route (which the relation does not provide directly). Finally, a markdown runbook documents deploying a single-model Canonical Identity Platform locally and exercising the login flow by hand.

**Tech Stack:** Go 1.25 (`internal/config`), Python 3.12 + `ops`/`paas-charm` (charm), YAML (`charmcraft.yaml`), MicroK8s + Juju 3.6 (manual verification).

## Global Constraints

- Module path: `bingo` (not `github.com/canonical/bingo`)
- Go binary path in this environment: `/home/daniel.nguyen@canonical.com/go/bin/go` (use `go` if already on `PATH`; verify with `go version` first)
- Run Go tests with: `go test ./...` (from repo root)
- Charm unit tests: from `charm/`, run `uv run --group unit pytest tests/unit -v` (requires `uv`; if not installed, `pip install --break-system-packages uv` or `python3 -m venv /tmp/toxvenv && /tmp/toxvenv/bin/pip install uv` then invoke `/tmp/toxvenv/bin/uv run --group unit pytest tests/unit -v` from `charm/`). Do **not** rely on the repo-root `tox.ini` for this — it has a pre-existing, unrelated path mismatch (expects `pyproject.toml`/`src`/`tests` at repo root, but they live under `charm/`) and fails with `error: No pyproject.toml found`. This is a known pre-existing issue, out of scope for this plan.
- TDD: write failing test → run to confirm FAIL → implement → run to confirm PASS → commit
- Commit co-author trailer on every commit: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`
- Design spec: `docs/superpowers/specs/2026-07-14-charm-oidc-verification-design.md` — refer back to it for the full rationale behind the env var fallback table and relation choice

---

## File Map

| File | Action | Responsibility |
|------|--------|-----------------|
| `internal/config/config.go` | Modify | Add `APP_OAUTH_*`/`APP_SECRET_KEY` fallback chain to `Load()`, derive redirect URL from base URL |
| `internal/config/config_test.go` | Modify | Add tests for the new fallback chain |
| `charm/charmcraft.yaml` | Modify | Add `oauth` relation + `oauth-redirect-path`/`oauth-scopes`/`oauth-user-name-attribute` config options |
| `docs/how-to-verify-oidc-locally.md` | Create | Manual runbook: deploy a local Identity Platform, relate it to bingo, verify the login flow by hand |

---

## Task 1: Go config OIDC fallback chain

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing new — builds on the existing `firstEnv(keys ...string) string`, `envOrDefault`, `valueOrDefault`, `parseIntValue` helpers already in `internal/config/config.go`.
- Produces: `config.Load()` continues to return `(*Config, error)` with the same `Config` struct shape (no fields added or renamed). Behavior change only: `OIDCIssuerURL`, `OIDCClientID`, `OIDCClientSecret`, `OIDCRedirectURL`, `SessionSecret` now resolve via a fallback chain instead of a single env var each.

- [ ] **Step 1: Write failing tests for the fallback chain**

Add to `internal/config/config_test.go` (after `TestLoad_OIDCEnabledRequiresSessionSecret`):

```go
func clearOIDCEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"OIDC_ISSUER_URL", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET", "OIDC_REDIRECT_URL",
		"SESSION_SECRET", "BASE_URL", "APP_BASE_URL",
		"APP_OAUTH_API_BASE_URL", "APP_OAUTH_CLIENT_ID", "APP_OAUTH_CLIENT_SECRET",
		"APP_SECRET_KEY",
	} {
		t.Setenv(k, "")
	}
}

func TestLoad_oauthRelationEnvVarsFallback(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv("APP_BASE_URL", "https://bingo.example.com")
	t.Setenv("APP_OAUTH_API_BASE_URL", "https://traefik-ip/model-hydra")
	t.Setenv("APP_OAUTH_CLIENT_ID", "bingo-oauth-client")
	t.Setenv("APP_OAUTH_CLIENT_SECRET", "hydra-issued-secret")
	t.Setenv("APP_SECRET_KEY", "charm-managed-secret-key")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"OIDCIssuerURL", cfg.OIDCIssuerURL, "https://traefik-ip/model-hydra"},
		{"OIDCClientID", cfg.OIDCClientID, "bingo-oauth-client"},
		{"OIDCClientSecret", cfg.OIDCClientSecret, "hydra-issued-secret"},
		{"OIDCRedirectURL", cfg.OIDCRedirectURL, "https://bingo.example.com/auth/callback"},
		{"SessionSecret", cfg.SessionSecret, "charm-managed-secret-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
	if !cfg.AuthEnabled() {
		t.Error("AuthEnabled() = false, want true when charm-provided oauth vars are fully set")
	}
}

func TestLoad_plainOIDCVarsTakePrecedenceOverCharmVars(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv("APP_BASE_URL", "https://ignored.example.com")
	t.Setenv("OIDC_ISSUER_URL", "https://plain-issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "plain-client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "plain-secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://plain.example.com/auth/callback")
	t.Setenv("SESSION_SECRET", "plain-session-secret")
	t.Setenv("APP_OAUTH_API_BASE_URL", "https://charm-issuer.example.com")
	t.Setenv("APP_OAUTH_CLIENT_ID", "charm-client-id")
	t.Setenv("APP_OAUTH_CLIENT_SECRET", "charm-secret")
	t.Setenv("APP_SECRET_KEY", "charm-session-secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"OIDCIssuerURL", cfg.OIDCIssuerURL, "https://plain-issuer.example.com"},
		{"OIDCClientID", cfg.OIDCClientID, "plain-client-id"},
		{"OIDCClientSecret", cfg.OIDCClientSecret, "plain-secret"},
		{"OIDCRedirectURL", cfg.OIDCRedirectURL, "https://plain.example.com/auth/callback"},
		{"SessionSecret", cfg.SessionSecret, "plain-session-secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestLoad_oauthFallbackMissingBaseURLReturnsPartialError(t *testing.T) {
	clearOIDCEnv(t)
	// Charm-provided client vars present, but no base URL to derive a redirect
	// from and no explicit OIDC_REDIRECT_URL — redirect stays empty, so the
	// all-or-nothing OIDC validation must reject this as partial configuration.
	t.Setenv("APP_OAUTH_API_BASE_URL", "https://traefik-ip/model-hydra")
	t.Setenv("APP_OAUTH_CLIENT_ID", "bingo-oauth-client")
	t.Setenv("APP_OAUTH_CLIENT_SECRET", "hydra-issued-secret")
	t.Setenv("APP_SECRET_KEY", "charm-managed-secret-key")

	_, err := config.Load()
	if err == nil {
		t.Error("Load() with no base URL to derive a redirect from: want error, got nil")
	}
}

func TestLoad_noOIDCVarsAtAllAuthDisabled(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv("APP_BASE_URL", "https://bingo.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AuthEnabled() {
		t.Error("AuthEnabled() = true, want false when no OIDC vars (plain or charm) are set")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/... -run TestLoad_oauth -v` and `go test ./internal/config/... -run TestLoad_plainOIDCVarsTakePrecedenceOverCharmVars -v` and `go test ./internal/config/... -run TestLoad_noOIDCVarsAtAllAuthDisabled -v`

Expected: `TestLoad_oauthRelationEnvVarsFallback` and `TestLoad_oauthFallbackMissingBaseURLReturnsPartialError` FAIL (fields stay empty since `Load()` doesn't yet read `APP_OAUTH_*`/`APP_SECRET_KEY`; the "missing base URL" test fails because `err` is `nil` when it should be non-nil). `TestLoad_plainOIDCVarsTakePrecedenceOverCharmVars` and `TestLoad_noOIDCVarsAtAllAuthDisabled` should already PASS (existing behavior) — that's fine, they're regression guards for the refactor in Step 3.

- [ ] **Step 3: Implement the fallback chain in `Load()`**

Replace the body of `Load()` in `internal/config/config.go`:

```go
// Load reads configuration from the environment, applying defaults where defined.
// Returns an error if any numeric variable is present but malformed.
func Load() (*Config, error) {
	// paas-charm's go-framework extension injects user-defined charm config
	// with an APP_ prefix; fall back to the unprefixed name for local/non-charm runs.
	maxSize, err := parseIntValue(firstEnv("APP_MAX_PASTE_SIZE_BYTES", "MAX_PASTE_SIZE_BYTES"), defaultMaxPasteSizeBytes)
	if err != nil {
		return nil, fmt.Errorf("MAX_PASTE_SIZE_BYTES: %w", err)
	}

	baseURL := firstEnv("APP_BASE_URL", "BASE_URL")

	// OIDC config sources, in priority order:
	//  1. Plain OIDC_*/SESSION_SECRET env vars (standalone/non-charm deployments).
	//  2. The charm's `oauth` relation to Charmed Hydra, which paas_charm injects
	//     as APP_OAUTH_* (client id/secret/issuer/endpoints) and APP_SECRET_KEY
	//     (a charm-managed, peer-secret-stored session secret). The relation does
	//     not provide a redirect URL, so it is derived from the base URL and the
	//     app's fixed callback route.
	oidcIssuerURL := firstEnv("OIDC_ISSUER_URL", "APP_OAUTH_API_BASE_URL")
	oidcClientID := firstEnv("OIDC_CLIENT_ID", "APP_OAUTH_CLIENT_ID")
	oidcClientSecret := firstEnv("OIDC_CLIENT_SECRET", "APP_OAUTH_CLIENT_SECRET")
	oidcRedirectURL := os.Getenv("OIDC_REDIRECT_URL")
	if oidcRedirectURL == "" && oidcIssuerURL != "" && oidcClientID != "" && oidcClientSecret != "" && baseURL != "" {
		oidcRedirectURL = strings.TrimSuffix(baseURL, "/") + "/auth/callback"
	}
	sessionSecret := firstEnv("SESSION_SECRET", "APP_SECRET_KEY")

	cfg := &Config{
		Port:              envOrDefault("PORT", "8080"),
		DatabaseURL:       envOrDefault("POSTGRESQL_DB_CONNECT_STRING", os.Getenv("DATABASE_URL")),
		MaxPasteSizeBytes: maxSize,
		BaseURL:           baseURL,
		LogLevel:          valueOrDefault(firstEnv("APP_LOG_LEVEL", "LOG_LEVEL"), "info"),
		OIDCIssuerURL:     oidcIssuerURL,
		OIDCClientID:      oidcClientID,
		OIDCClientSecret:  oidcClientSecret,
		OIDCRedirectURL:   oidcRedirectURL,
		SessionSecret:     sessionSecret,
		WebDir:            firstEnv("APP_WEB_DIR", "WEB_DIR"),
	}

	// Validate OIDC config: either all-or-nothing, and SESSION_SECRET required.
	oidcCount := 0
	for _, v := range []string{cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.OIDCRedirectURL} {
		if v != "" {
			oidcCount++
		}
	}
	if oidcCount > 0 && oidcCount < 4 {
		return nil, fmt.Errorf("partial OIDC configuration: all four OIDC_* variables must be set together")
	}
	if oidcCount == 4 && cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required when OIDC is configured")
	}

	return cfg, nil
}
```

Add `"strings"` to the import block at the top of the file:

```go
import (
	"fmt"
	"os"
	"strconv"
	"strings"
)
```

- [ ] **Step 4: Run all config tests to verify they pass**

Run: `go test ./internal/config/... -v`

Expected: all tests PASS, including the 4 pre-existing tests (`TestLoad_defaults`, `TestLoad_fromEnv`, `TestLoad_invalidMaxPasteSize`, `TestConfig_AuthEnabled_false`, `TestConfig_AuthEnabled_true`, `TestLoad_partialOIDCReturnsError`, `TestLoad_OIDCEnabledRequiresSessionSecret`) and the 4 new ones added in Step 1.

- [ ] **Step 5: Run the full Go test suite to check for regressions**

Run: `go test ./...`

Expected: PASS (no other package reads `config.Config` fields in a way that would be affected — this change only affects how the fields are populated, not their type or names).

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "$(cat <<'EOF'
Add APP_OAUTH_*/APP_SECRET_KEY fallback chain to config.Load()

Lets OIDC config resolve from paas_charm's oauth relation env vars
(APP_OAUTH_CLIENT_ID, APP_OAUTH_CLIENT_SECRET, APP_OAUTH_API_BASE_URL,
APP_SECRET_KEY) when the plain OIDC_*/SESSION_SECRET vars are unset,
deriving the redirect URL from APP_BASE_URL + /auth/callback since the
relation provides no redirect URL of its own. Plain env vars still take
precedence for standalone/non-charm deployments.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 2: Charm `oauth` relation + config options

**Files:**
- Modify: `charm/charmcraft.yaml`

**Interfaces:**
- Consumes: nothing from Task 1 (charm and Go config changes are independent; both read/write the same env var contract but neither calls the other's code directly).
- Produces: a `bingo:oauth` relation endpoint (interface `oauth`) that `juju integrate bingo:oauth hydra:oauth` can target, plus `oauth-redirect-path`, `oauth-scopes`, `oauth-user-name-attribute` charm config options consumed by `paas_charm.go.Charm`'s built-in `PaaSOAuthRequirer`.

- [ ] **Step 1: Establish the pre-change baseline for charm unit tests**

Run (from `charm/`):

```bash
cd charm
uv run --group unit pytest tests/unit -v
```

Expected: `4 passed` (the existing `test_charm_initialises`, `test_pebble_ready_without_postgresql`, `test_pebble_not_connected`, `test_config_changed_invalid_log_level`). This confirms the toolchain works before making changes, so any failure after Step 3 is attributable to this task's edit.

- [ ] **Step 2: Add the `oauth` relation to `requires`**

In `charm/charmcraft.yaml`, edit the `requires` block:

```yaml
requires:
  postgresql:
    interface: postgresql_client
    limit: 1
  tracing:
    interface: tracing
    optional: true
    limit: 1
  oauth:
    interface: oauth
    optional: true
    limit: 1
```

- [ ] **Step 3: Add the three `oauth-*` config options**

In `charm/charmcraft.yaml`, add to the end of `config.options` (after `web-dir`):

```yaml
    oauth-redirect-path:
      type: string
      description: |
        Path appended to base-url to build the OIDC callback redirect URI
        registered with the identity provider over the oauth relation. Must
        match the Go app's callback route (/auth/callback).
      default: /auth/callback
    oauth-scopes:
      type: string
      description: |
        Space-separated OAuth scopes requested from the identity provider over
        the oauth relation. Must include 'openid'.
      default: "openid email profile"
    oauth-user-name-attribute:
      type: string
      description: |
        Claim used to identify the authenticated user, from the identity
        provider's userinfo response, over the oauth relation.
      default: sub
```

- [ ] **Step 4: Validate `charmcraft.yaml` is well-formed YAML with the expected keys**

Run:

```bash
cd charm
python3 -c "
import yaml
d = yaml.safe_load(open('charmcraft.yaml'))
assert d['requires']['oauth'] == {'interface': 'oauth', 'optional': True, 'limit': 1}, d['requires']['oauth']
opts = d['config']['options']
assert opts['oauth-redirect-path']['default'] == '/auth/callback'
assert 'openid' in opts['oauth-scopes']['default']
assert opts['oauth-user-name-attribute']['default'] == 'sub'
print('charmcraft.yaml OK')
"
```

Expected output: `charmcraft.yaml OK`

- [ ] **Step 5: Re-run charm unit tests to confirm no regression**

Run (from `charm/`):

```bash
cd charm
uv run --group unit pytest tests/unit -v
```

Expected: `4 passed` (same as Step 1 baseline — declaring an optional relation and new config options with defaults must not change existing charm behavior, since nothing currently relates `oauth` or sets the new config keys).

- [ ] **Step 6: Commit**

```bash
git add charm/charmcraft.yaml
git commit -m "$(cat <<'EOF'
Add oauth relation and config options to bingo charm

Declares an optional `oauth` relation (interface `oauth`, matching the
vendored charms.hydra.v0.oauth library) so bingo can be related to
Charmed Hydra for OIDC authentication. Adds oauth-redirect-path (default
/auth/callback, matching the Go app's callback route),
oauth-scopes, and oauth-user-name-attribute config options that
paas_charm's PaaSOAuthRequirer reads to register bingo as an OAuth
client with hydra.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 3: Manual local verification runbook

**Files:**
- Create: `docs/how-to-verify-oidc-locally.md`

**Interfaces:**
- Consumes: the `bingo:oauth` relation and config options from Task 2, and the env var fallback chain from Task 1 (the runbook's success criteria depend on both being in place).
- Produces: a standalone markdown runbook; no code interfaces.

- [ ] **Step 1: Write the runbook**

Create `docs/how-to-verify-oidc-locally.md`:

```markdown
# How to verify OIDC authentication locally on a deployed bingo charm

This is a manual, one-off verification procedure — not an automated test. It
confirms that a deployed bingo charm can authenticate users via the Canonical
Identity Platform (Charmed Hydra) over the `oauth` Juju relation.

## Prerequisites

- A Juju controller (v3.6+) bootstrapped on MicroK8s with the `dns` and
  `hostpath-storage` addons enabled (see `charm/concierge.yaml` for the same
  setup used by CI).
- `charmcraft pack` has produced a `bingo_*.charm` file, and an OCI image for
  the `app-image` resource is available (see `charm/tests/integration/test_charm.py`
  for the equivalent CI build/deploy pattern).

## 1. Deploy bingo with its existing dependencies

In a single Juju model (`local-oidc-check` here), deploy bingo the same way
`charm/tests/integration/test_charm.py::test_build_and_deploy` does:

```bash
juju add-model local-oidc-check
juju deploy ./bingo_*.charm --resource app-image=<your-oci-image-ref> --trust
juju deploy postgresql-k8s --channel 14/stable --trust
juju integrate bingo:postgresql postgresql:database
juju deploy traefik-k8s --channel latest/stable --trust
juju integrate bingo:ingress traefik:ingress
juju wait-for application bingo --query='status=="active"'
```

## 2. Deploy a local Canonical Identity Platform

Deploy hydra, kratos, the login UI, and their shared dependencies into the
**same model** (collapsing the production `core`/`iam` two-model split from
Canonical's tutorial into one model, since this is a local one-off check, not
a production topology):

```bash
juju deploy self-signed-certificates --channel 1/stable
juju deploy hydra --channel latest/stable --trust
juju deploy kratos --channel latest/stable --trust
juju deploy identity-platform-login-ui-operator --channel latest/stable --trust

juju integrate hydra:pg-database postgresql:database
juju integrate kratos:pg-database postgresql:database
juju integrate hydra:public-ingress traefik:ingress
juju integrate hydra:admin-ingress traefik:ingress
juju integrate kratos:public-ingress traefik:ingress
juju integrate hydra:ui-endpoint-info identity-platform-login-ui-operator:ui-endpoint-info
juju integrate kratos:ui-endpoint-info identity-platform-login-ui-operator:ui-endpoint-info
juju integrate hydra:hydra-endpoint-info kratos:hydra-endpoint-info
juju integrate hydra:certificates self-signed-certificates:certificates
juju integrate kratos:certificates self-signed-certificates:certificates

juju wait-for application hydra --query='status=="active"'
juju wait-for application kratos --query='status=="active"'
```

Reference: Canonical's ["Get started with the Canonical Identity
Platform"](https://canonical-identity.readthedocs-hosted.com/tutorial/canonical-identity-platform/)
tutorial documents the full production (two-model, Terraform-driven) version
of this deployment — use it if any relation above needs adjusting for your
Juju/charm revision.

## 3. Relate bingo to hydra

```bash
juju integrate bingo:oauth hydra:oauth
juju wait-for application bingo --query='status=="active"'
```

This triggers `paas_charm`'s `PaaSOAuthRequirer` to register bingo as an OAuth
client with hydra, using the `oauth-redirect-path` (`/auth/callback` by
default) and `oauth-scopes` (`openid email profile` by default) config values
from Task 2. Confirm the registration happened:

```bash
juju exec --application hydra -- hydra list oauth2-clients
```

You should see a client entry whose `redirect_uris` includes
`<bingo base-url>/auth/callback`.

## 4. Exercise the login flow

```bash
juju config bingo base-url=http://<traefik-ip>/local-oidc-check-bingo
juju status --relations  # confirm bingo, hydra, kratos, traefik are all active/idle
```

In a browser:

1. Visit `http://<traefik-ip>/local-oidc-check-bingo/auth/login`.
2. You should be redirected to hydra's authorization endpoint, then to
   Kratos's self-service login/registration UI.
3. Register a test user (any email/password) and complete the flow.
4. Confirm you land back on bingo with a session cookie set
   (`bingo_session` or equivalent — check dev tools → Application → Cookies).

## 5. Verify the authenticated session end-to-end

```bash
curl -b cookies.txt -c cookies.txt http://<traefik-ip>/local-oidc-check-bingo/api/v1/me
```

Expected: a JSON body with your test user's `sub`/`email`, not a `401`.

Create a paste while authenticated (via the browser UI or an authenticated
`curl` request with the session cookie) and confirm it is attributed to your
user — e.g. it appears in the "my pastes" listing and is deletable by you but
would be rejected (`403`) for a different user's session.

## Fallback: manual OAuth client registration

If `juju integrate bingo:oauth hydra:oauth` doesn't produce a working client
(e.g. relation data isn't populating as expected), register the client
manually as a diagnostic cross-check:

```bash
juju run hydra/leader create-oauth-client \
  grant-types='["authorization_code"]' \
  redirect-uris='["http://<traefik-ip>/local-oidc-check-bingo/auth/callback"]' \
  response-types='["code"]' \
  scope='["openid","profile","email"]' \
  token-endpoint-auth-method="client_secret_basic"
```

Then set the plain `OIDC_*` env vars directly (bypassing the relation) to
isolate whether the issue is in the relation wiring or the OIDC flow itself.
This is not part of the primary verification path — only use it to narrow
down a failure.
```

- [ ] **Step 2: Cross-check the runbook against the design spec**

Re-read `docs/superpowers/specs/2026-07-14-charm-oidc-verification-design.md`
section 3 side-by-side with the new runbook. Confirm every command in the
runbook corresponds to a step described in the spec (relation name `oauth`,
single-model topology, `create-oauth-client` fallback) and that no step
references a config option or env var not defined in Task 1/Task 2. Fix any
mismatch inline.

- [ ] **Step 3: Commit**

```bash
git add docs/how-to-verify-oidc-locally.md
git commit -m "$(cat <<'EOF'
Add manual runbook for verifying OIDC on a deployed bingo charm

Documents deploying a single-model local Canonical Identity Platform
(hydra + kratos + login-ui + self-signed-certificates), relating it to
bingo's new oauth relation, and exercising the browser-based login flow
end-to-end, with a create-oauth-client fallback for diagnosing relation
issues.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review Notes

- **Spec coverage:** Task 1 implements the design spec's "Go config adaptation"
  section fallback table exactly (issuer/client id/secret/redirect/session
  secret, each with the documented primary/fallback source). Task 2 implements
  the "Charm changes" section (relation + 3 config options, verbatim defaults).
  Task 3 implements the "Local manual verification procedure" section,
  including the `create-oauth-client` fallback called out in the design's
  Q&A. The design's "Open Questions/Risks" (channel pinning, CA trust) are
  intentionally left as runbook-time judgment calls, not pre-baked into fixed
  commands, since they depend on the operator's environment at verification
  time.
- **No placeholders:** every step has literal, runnable commands/code — no
  "TODO"/"similar to above" left in any task.
- **Type consistency:** `Config` struct fields (`OIDCIssuerURL`, `OIDCClientID`,
  `OIDCClientSecret`, `OIDCRedirectURL`, `SessionSecret`) are unchanged in name
  and type throughout; only their population logic changes. `AuthEnabled()` is
  untouched and continues to operate on these same field names.
