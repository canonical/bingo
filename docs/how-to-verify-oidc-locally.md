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
