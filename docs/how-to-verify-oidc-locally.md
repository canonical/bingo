# How to verify OIDC authentication locally on a deployed bingo charm

This is a manual, one-off verification procedure — not an automated test. It
confirms that the `oauth` Juju relation between the bingo charm and the
Hydra charm actually results in a working OIDC login, end to end.

It deliberately uses the **smallest topology that exercises the real relation
code path**: no Kratos, no `self-signed-certificates`, and no browser-driven
login form. Charmed Hydra normally sits behind Kratos and a login UI backed
by a real identity backend — none of that is needed to prove that bingo's
`oauth` relation and `paas_charm`'s `PaaSOAuthRequirer` wiring work, because
Hydra's login/consent challenges can be accepted directly through its admin
API, the same way Ory's own Hydra test suite does it.

## Why this topology and not the full Canonical Identity Platform

| Component | Needed here? | Why |
| --- | --- | --- |
| `hydra` | Yes | It's the actual OIDC provider bingo's `oauth` relation talks to (`charms.hydra.v0.oauth` interface). |
| `identity-platform-login-ui-operator` | Yes, but only as a placeholder | Hydra's `ui-endpoint-info` relation is `optional: false` — Hydra won't go active without something providing it. `login-ui`'s own `kratos-info` relation is optional, so it works standalone. |
| `kratos` | **No** | Only needed if you want a real credential-checking login form. We accept login/consent via Hydra's admin API instead. |
| `self-signed-certificates` | **No** | Set `juju config hydra dev=true` instead — Hydra's dev mode skips the HTTPS requirement. |
| `traefik-k8s` | Yes | Hydra's `public-route` relation (`traefik-route` interface) is `optional: false`, and bingo needs `ingress` too. |
| `postgresql-k8s` | Yes | Backing store for both bingo and Hydra. |

## Prerequisites

- A Juju controller (v3.6+) bootstrapped on MicroK8s with the `dns` and
  `hostpath-storage` addons enabled (see `charm/concierge.yaml`).
- `charmcraft pack` has produced a `bingo_*.charm` file, and an OCI image for
  the `app-image` resource is available (see
  `charm/tests/integration/test_charm.py` for the equivalent CI pattern).
- `curl` and `jq` installed locally.

## 1. Deploy bingo with its existing dependencies

```bash
juju add-model local-oidc-check
juju deploy ./bingo_*.charm --resource app-image=<your-oci-image-ref> --trust
juju deploy postgresql-k8s --channel 14/stable --trust
juju integrate bingo:postgresql postgresql:database

juju deploy traefik-k8s --channel latest/stable --trust
juju integrate bingo:ingress traefik:ingress

juju wait-for application bingo --query='status=="active"'
```

## 2. Deploy the minimal Hydra topology (no Kratos, no certs)

```bash
juju deploy hydra --channel latest/stable --trust
juju config hydra dev=true

juju deploy identity-platform-login-ui-operator --channel latest/stable --trust

juju integrate hydra:pg-database postgresql:database
juju integrate hydra:ui-endpoint-info identity-platform-login-ui-operator:ui-endpoint-info
juju integrate hydra:public-route traefik:traefik-route

juju wait-for application hydra --query='status=="active"'
juju wait-for application identity-platform-login-ui-operator --query='status=="active"'
```

## 3. Relate bingo to hydra

```bash
juju integrate bingo:oauth hydra:oauth
juju wait-for application bingo --query='status=="active"'
```

This triggers `paas_charm`'s `PaaSOAuthRequirer` to register bingo as an OAuth
client with hydra, using the `oauth-redirect-path` (`/auth/callback` by
default), `oauth-scopes` (`openid email profile` by default), and
`oauth-user-name-attribute` (`sub` by default) config values. Confirm the
registration happened:

```bash
juju run hydra/leader list-oauth-clients
```

You should see a client entry whose `redirect_uris` includes
`<bingo base-url>/auth/callback`.

```bash
juju config bingo base-url=http://<traefik-ip>/local-oidc-check-bingo
```

## 4. Reach Hydra's admin API

The admin API is cluster-internal; port-forward it before running the script
below:

```bash
kubectl port-forward -n local-oidc-check svc/hydra-admin 4445:4445
```

## 5. Drive the login flow with `verify-oidc-flow.sh` (no browser needed)

Save this as `verify-oidc-flow.sh`, fill in the two variables at the top, and
run it. It plays the role of a browser + a real login UI: it captures the
`login_challenge`/`consent_challenge` Hydra generates and accepts them
directly via the admin API, rather than following the redirect to a real
login form.

```bash
#!/usr/bin/env bash
set -euo pipefail

# --- fill these in ---
BINGO_BASE_URL="http://<traefik-ip>/local-oidc-check-bingo"
HYDRA_ADMIN_URL="http://localhost:4445"   # via the kubectl port-forward above
TEST_SUBJECT="test-user@example.com"
# ---------------------

COOKIES="$(mktemp)"
trap 'rm -f "$COOKIES"' EXIT

location() {
  # Print the Location header from a response without following it.
  curl -s -D - -o /dev/null -c "$COOKIES" -b "$COOKIES" "$1" \
    | grep -i '^location:' | sed 's/^[Ll]ocation: *//; s/\r$//'
}

echo "==> Hitting bingo's /auth/login"
hydra_authorize_url="$(location "$BINGO_BASE_URL/auth/login")"
echo "    -> $hydra_authorize_url"

echo "==> Following into Hydra's /oauth2/auth to get a login_challenge"
login_redirect="$(location "$hydra_authorize_url")"
login_challenge="$(echo "$login_redirect" | grep -oP 'login_challenge=\K[^&]+')"
echo "    login_challenge=$login_challenge"

echo "==> Accepting the login challenge via Hydra's admin API (no real login form)"
login_accept="$(curl -s -X PUT \
  "$HYDRA_ADMIN_URL/admin/oauth2/auth/requests/login/accept?login_challenge=$login_challenge" \
  -H 'Content-Type: application/json' \
  -d "{\"subject\": \"$TEST_SUBJECT\", \"remember\": false}")"
consent_entry_url="$(echo "$login_accept" | jq -r '.redirect_to')"

echo "==> Following back into Hydra to get a consent_challenge"
consent_redirect="$(location "$consent_entry_url")"
consent_challenge="$(echo "$consent_redirect" | grep -oP 'consent_challenge=\K[^&]+')"
echo "    consent_challenge=$consent_challenge"

echo "==> Accepting the consent challenge, granting the requested scopes"
consent_accept="$(curl -s -X PUT \
  "$HYDRA_ADMIN_URL/admin/oauth2/auth/requests/consent/accept?consent_challenge=$consent_challenge" \
  -H 'Content-Type: application/json' \
  -d '{
        "grant_scope": ["openid", "email", "profile"],
        "grant_access_token_audience": [],
        "session": {"id_token": {"email": "'"$TEST_SUBJECT"'", "sub": "'"$TEST_SUBJECT"'"}}
      }')"
final_redirect_url="$(echo "$consent_accept" | jq -r '.redirect_to')"

echo "==> Following the final redirect chain back to bingo's /auth/callback"
curl -s -o /dev/null -c "$COOKIES" -b "$COOKIES" -L "$final_redirect_url"

echo "==> Verifying the authenticated session"
curl -s -b "$COOKIES" "$BINGO_BASE_URL/api/v1/me" | jq .
```

```bash
chmod +x verify-oidc-flow.sh
./verify-oidc-flow.sh
```

Expected final output: a JSON body with `"sub": "test-user@example.com"` (or
equivalent), not a `401`.

## 6. Verify the authenticated session end-to-end

Using the same cookie jar the script populated (or the `Set-Cookie` value
from step 5), create a paste while authenticated and confirm it's attributed
to the test user — e.g. it appears in the "my pastes" listing and would be
rejected (`403`) for a different session's user.

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

## Full browser-based verification with Kratos (real login UI)

The admin-API-driven flow above is enough to prove the `oauth` relation and
OIDC code path work, but it never exercises a real browser: no cookies set
by an actual login form, no path-prefix/reverse-proxy quirks, no TLS trust
chain. Use this heavier topology when you need to click through the actual
login/logout UI in a browser — e.g. to verify a charm-level fix that only
manifests under a real browser (redirect handling, cookie `Path`/`SameSite`,
CSP, asset loading through a path-prefixed ingress).

### Topology

In addition to everything in the minimal setup above, deploy:

```bash
juju deploy self-signed-certificates --channel 1/stable
juju deploy kratos --channel latest/stable --trust
juju deploy postgresql-k8s --channel 14/stable --trust  # if not already deployed

juju integrate kratos:pg-database postgresql:database
juju integrate kratos:public-ingress traefik:ingress
juju integrate kratos:certificates self-signed-certificates:certificates
juju integrate kratos:ui-endpoint-info identity-platform-login-ui-operator:ui-endpoint-info
juju integrate kratos:hydra-endpoint-info hydra:hydra-endpoint-info

# Easy to miss — login-ui needs Hydra's admin API URL directly, separate
# from the ui-endpoint-info relation. Without this, login silently loops
# back to a confusing "?aal=aal2" re-auth prompt (see "Gotchas" below).
juju integrate hydra:hydra-endpoint-info identity-platform-login-ui-operator:hydra-endpoint-info

juju config hydra dev=false   # see "Gotchas" below — dev=true breaks the callback
juju config kratos dev=true enforce_mfa=false
```

Trust the deployment's self-signed CA locally (or in the app container, if
testing from inside the same cluster) so TLS handshakes succeed:

```bash
juju run self-signed-certificates/leader get-ca-certificate | \
  yq -r '.["ca-certificate"]' >> /etc/ssl/certs/ca-certificates.crt
```

### Creating a test identity

Canonical Identity Platform's Kratos deployment has no self-service sign-up
UI reachable from `identity-platform-login-ui-operator` — create identities
via Kratos's admin API instead:

```bash
kubectl port-forward -n <model> svc/kratos-admin 4434:4434
curl -s -X POST http://localhost:4434/admin/identities \
  -H 'Content-Type: application/json' \
  -d '{
        "schema_id": "social_user_v0",
        "traits": {"email": "browser-user@example.com"}
      }'
```

**Use `schema_id: social_user_v0`, not `admin_v0`.** Identities created under
`admin_v0` are treated as requiring AAL2 (a second factor), which manifests
as an infinite-looking redirect back to `/ui/login?aal=aal2` after a
seemingly-successful password login — with no actual MFA prompt or way
forward. This is a schema choice, not a bug; `social_user_v0` behaves like a
normal single-factor user.

Set the identity's password via Kratos's self-service recovery flow (there's
no direct "set password" admin endpoint) — request a recovery link via the
admin API, then complete it through `/self-service/recovery/browser` in the
browser to set a real password.

### Gotchas encountered testing this way

These were all found and fixed while verifying bingo's charm-level OIDC
integration with this topology — recorded here so they don't need
rediscovering:

1. **`oidc_state` cookie `Path`.** Bingo originally scoped this cookie to
   `/auth/callback`. Under Traefik's path-prefix ingress, the browser sees
   `/<prefix>/auth/callback`, so a cookie scoped to the unprefixed path is
   never sent back — surfacing as `{"error":{"code":"invalid_state", ...
   "OIDC state mismatch — possible CSRF."}}`. Fixed by scoping it to `/`.
2. **Hydra `dev=true` cookies.** In dev mode Hydra sets its CSRF/session
   cookies with `SameSite=None` but without `Secure`, which browsers reject
   over HTTPS — the callback then fails with `{"error":{"code":
   "invalid_request","message":"Missing authorization code."}}`. Use
   `juju config hydra dev=false` for any HTTPS-fronted test, even locally.
3. **Missing `hydra:hydra-endpoint-info` ↔ `login-ui` relation.** Without
   it, login-ui can't reach Hydra's admin API to process consent
   (`unsupported protocol scheme ""` in login-ui's logs) and the symptom to
   the user is a confusing redirect back to `/ui/login?aal=aal2` — a red
   herring that looks like a step-up-auth requirement but is really a
   missing relation. See "Topology" above.
4. **`admin_v0` vs `social_user_v0` schema.** See "Creating a test identity"
   above.
5. **Path-prefix breaks absolute URLs everywhere.** Traefik's default
   ingress-per-app mode strips the app's path prefix before forwarding, but
   the browser only ever sees the full prefixed URL. Any server-side
   redirect or frontend asset/API reference built as an absolute path
   (`/auth/login`, `/assets/...`, `/api/v1/...`) resolves against the
   domain root and 404s. Bingo now derives its external prefix from
   `cfg.BasePath()` (parsed from `APP_BASE_URL`, the same paas-charm
   go-framework-extension env var already used for paste URLs) everywhere:
   server-side redirects (`homeURL`, `postLogoutRedirectURL`, the
   `requireAuthMiddleware` login redirect), and an injected
   `<base href>` tag plus relative frontend paths for the SPA. This affects
   **real production deployments**, not just this local topology, unless a
   subdomain-per-app ingress is configured instead of path-based routing.
6. **Logout: local session clearing isn't enough for a "real" logout.**
   Since the whole app is gated behind auth, clearing bingo's own session
   cookie alone causes an immediate redirect back to `/auth/login` — and if
   the browser still holds a valid Kratos SSO session, Hydra silently
   re-authenticates it, so logout appears to do nothing. Bingo now
   implements standard OIDC RP-initiated logout (`Provider.LogoutURL`,
   using the `end_session_endpoint` from OIDC discovery plus an
   `id_token_hint` retained from login) — this requires the OAuth2 client's
   `post_logout_redirect_uris` to be registered with Hydra (not currently
   automated by paas-charm's `oauth` relation; register it manually for
   local testing via `PATCH /admin/clients/<id>`).
7. **Logout consent UI may not exist.** Hydra's RP-initiated logout flow
   unconditionally redirects the browser to a "logout consent" UI page
   (`urls.logout`, wired via the `hydra:ui-endpoint-info` relation to
   `identity-platform-login-ui-operator`) before it will finish processing
   a logout request — this happens regardless of the client's
   `skip_logout_consent` flag. As of `identity-platform-login-ui-operator`
   revision 197 (`latest/stable`), **no such page exists** (`/ui/logout`
   404s, and its relation databag never advertises a `logout_url`), so the
   full logout round-trip currently ends at Hydra's
   `urls.logout is not set` fallback error page in this topology. This is a
   known limitation of the current identity-platform-login-ui-operator
   revision, not a bug in bingo — the app's logout code is spec-correct and
   will complete successfully once paired with a login-ui revision (or
   different login UI) that implements this page.
