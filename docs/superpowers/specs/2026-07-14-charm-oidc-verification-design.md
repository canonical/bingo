# Charm OIDC Integration & Local Verification — Design

## Goal

Enable the deployed bingo charm to be configured for OIDC authentication via the
Canonical Identity Platform (`hydra`, using the `oauth` Juju relation interface), and
provide a repeatable local procedure to manually verify the full browser-based
authorization code flow against a real Identity Platform deployment.

Today, `internal/auth` fully supports OIDC when the Go binary is run directly with
`OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL`, and
`SESSION_SECRET` set (already verified working standalone). The charm, however, has
no mechanism to deliver this configuration: `charmcraft.yaml` declares no `oauth`
relation, and `paas_charm`'s go-framework extension — even if related — emits env
vars (`APP_OAUTH_CLIENT_ID`, `APP_OAUTH_API_BASE_URL`, discrete endpoint URLs) that
don't match the Go app's expected variable names, and provides neither a redirect URL
nor a session secret automatically.

## Non-Goals

- Automated integration test coverage (explicitly deferred; this is a manual,
  one-off local verification).
- Support for OIDC providers other than Charmed Hydra via the `oauth` interface (the
  plain `OIDC_*` env var path already covers arbitrary/non-charm OIDC providers).
- Multi-model / cross-model-offer Identity Platform topology (production topology);
  local verification uses a single Juju model for simplicity.

## Architecture

### 1. Charm relation (`charm/charmcraft.yaml`)

Add an optional `requires` relation:

```yaml
requires:
  oauth:
    interface: oauth
    optional: true
    limit: 1
```

This matches `charms.hydra.v0.oauth`'s `DEFAULT_RELATION_NAME` ("oauth"), already
vendored at `charm/lib/charms/hydra/v0/oauth.py`. `paas_charm.go.Charm` detects any
`requires` endpoint using interface `oauth` automatically (`get_endpoints_by_interface_name`)
and manages the relation lifecycle — no `charm.py` code changes are needed.

Add three config options that `paas_charm`'s `PaaSOAuthRequirer` reads to build the
OAuth client registration it sends to hydra:

```yaml
config:
  options:
    oauth-redirect-path:
      type: string
      description: |
        Path appended to base-url to build the OIDC callback redirect URI registered
        with the identity provider. Must match the Go app's callback route.
      default: /auth/callback
    oauth-scopes:
      type: string
      description: |
        Space-separated OAuth scopes requested from the identity provider. Must
        include 'openid'.
      default: "openid email profile"
    oauth-user-name-attribute:
      type: string
      description: Claim used to identify the authenticated user.
      default: sub
```

When related, `paas_charm` calls hydra's OAuth2 client registration API with
`redirect_uri = base-url + oauth-redirect-path`, then injects into the workload
container:

| Env var | Source |
|---|---|
| `APP_OAUTH_CLIENT_ID` | hydra-issued client ID |
| `APP_OAUTH_CLIENT_SECRET` | hydra-issued client secret |
| `APP_OAUTH_API_BASE_URL` | hydra's issuer URL |
| `APP_OAUTH_AUTHORIZE_URL` / `APP_OAUTH_ACCESS_TOKEN_URL` / `APP_OAUTH_USER_URL` / `APP_OAUTH_JWKS_URL` | hydra's discrete OIDC endpoints |
| `APP_SECRET_KEY` | paas_charm's own auto-generated, peer-relation-stored charm secret (always present, framework-managed) |

No env var is provided for the redirect URL or a session-specific secret.

### 2. Go config adaptation (`internal/config/config.go`, `internal/auth/provider.go`)

Extend `config.Load()` so each OIDC-related field falls back to the charm-provided
value when the plain env var is unset:

| Field | Primary source | Fallback source |
|---|---|---|
| `OIDCIssuerURL` | `OIDC_ISSUER_URL` | `APP_OAUTH_API_BASE_URL` |
| `OIDCClientID` | `OIDC_CLIENT_ID` | `APP_OAUTH_CLIENT_ID` |
| `OIDCClientSecret` | `OIDC_CLIENT_SECRET` | `APP_OAUTH_CLIENT_SECRET` |
| `OIDCRedirectURL` | `OIDC_REDIRECT_URL` | derived: `APP_BASE_URL` (trailing slash trimmed) + `/auth/callback` |
| `SessionSecret` | `SESSION_SECRET` | `APP_SECRET_KEY` |

`AuthEnabled()`'s validation (all-or-nothing OIDC fields, session secret required)
runs against the *resolved* values, after fallback substitution, so behavior for the
plain-env-var (standalone) path is unchanged. `internal/auth/provider.go` requires no
changes — `gooidc.NewProvider(ctx, cfg.OIDCIssuerURL)` performs discovery against
hydra's public issuer URL (`APP_OAUTH_API_BASE_URL`) exactly as it does for any other
OIDC-compliant issuer today.

### 3. Local manual verification procedure

Documented as a runbook (not automated). Deploy, in a single MicroK8s/Juju model
alongside the existing bingo + `postgresql-k8s` + `traefik-k8s` deployment (per
`charm/tests/integration/test_charm.py`'s existing pattern):

- `self-signed-certificates`, `postgresql-k8s` (shared or second instance),
  `hydra`, `kratos`, `identity-platform-login-ui-operator` — component list and
  relations adapted from Canonical's ["Get started with the Canonical Identity
  Platform"](https://canonical-identity.readthedocs-hosted.com/tutorial/canonical-identity-platform/)
  tutorial, collapsed into one model instead of the tutorial's `core`/`iam` split.
- `juju integrate bingo:oauth hydra:oauth` (per ["Integrate your OIDC-compatible
  charm"](https://canonical-identity.readthedocs-hosted.com/how-to/integrate-with-oidc-compatible-charms/)).
- Wait for `juju status` to show all units `active`/`idle`.
- Verify by browser: visit `https://<traefik-ip>/bingo/auth/login`, complete Kratos's
  self-service registration/login flow, confirm redirect back to bingo sets a session
  cookie, and that `GET /api/v1/me` plus paste creation reflect the authenticated
  identity (owner attribution).
- Fallback if relation-based auto-registration misbehaves: register the OAuth client
  manually via `juju run hydra/leader create-oauth-client ...` (per the ["Onboard an
  application with Charmed
  Hydra"](https://canonical-identity.readthedocs-hosted.com/how-to/onboard-an-application-with-charmed-hydra/)
  guide) and set the plain `OIDC_*` charm config directly as a diagnostic
  cross-check — not part of the primary verification path.

## Testing

- Unit tests for the new `config.Load()` fallback chain (charm-provided env vars
  present but plain `OIDC_*`/`SESSION_SECRET` absent → fields resolve correctly;
  plain vars present → charm vars ignored; neither present → auth disabled).
- No new integration/spread tests (explicitly out of scope — manual verification
  only, per user decision).
- Manual verification runbook above serves as the acceptance check for this task.

## Open Questions / Risks

- Exact component versions/channels for `hydra`/`kratos`/`identity-platform-login-ui-operator`
  should be pinned to `latest/stable` or whatever channel matches Juju 3.6
  compatibility at implementation time; verify during execution rather than baking
  a specific revision into this spec.
- If hydra requires HTTPS-only issuer URLs for OIDC discovery to succeed, the
  `self-signed-certificates` charm's CA must be trusted by the Go binary's HTTP
  client (or discovery will fail on cert validation) — confirm during
  implementation and adjust (e.g. inject the CA bundle into the workload
  container) if needed.
