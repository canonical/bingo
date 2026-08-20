---
myst:
  html_meta:
    "description lang=en": "Learn how to configure OIDC login for the bingo charm using the oauth relation."
---

(how_to_configure_oidc_login)=

# How to configure OIDC login

The bingo charm exposes configurations for OpenID Connect (OIDC) authentication over the `oauth` relation, enabling you to restrict access to trusted users, enforce single sign-on with your existing identity provider, and secure your pastebin deployment against unauthorized use.

## Prerequisites

This guide assumes you already have a working, OIDC-compliant identity provider deployed and
integrated with bingo over the `oauth` relation — for example the
[Canonical Identity Platform](https://charmhub.io/identity-platform).

## Configure OAuth scopes and the username attribute

This charm exposes two configuration options that are sent as metadata to the identity provider
when registering bingo as an OAuth client:

- `oauth-scopes`: space-separated OAuth scopes requested from the identity provider. Must include
  `openid`. Default: `"openid email profile"`.
- `oauth-user-name-attribute`: claim from the identity provider's userinfo response used to
  identify the authenticated user. Default: `sub`.

Set the configurations:

```
juju config bingo oauth-scopes="openid email profile"
juju config bingo oauth-user-name-attribute=email
```

These options only affect the client registration recorded with the identity provider — bingo's
own OIDC scopes are fixed internally to `openid email profile`, so changing them does not change
what the application requests at runtime.

```{warning}
Do not change `oauth-redirect-path`: bingo's callback route is hardcoded to `/auth/callback`, and
the charm blocks the unit if this option is overridden.
```

## Verify

Check that bingo's `oauth` relation is active and the charm is not blocked:

```{terminal}
:copy:

juju status --relations

App             Status  Scale  Charm  Channel  Rev  Address  Exposed  Message
bingo           active      1  bingo                  <ip>     no
hydra           active      1  hydra                  <ip>     no
...

Integration provider  Requirer     Interface  Type     Message
hydra:oauth           bingo:oauth  oauth      regular
...
```

Navigate to bingo's base URL in a browser and confirm you are redirected to the identity
provider's login page, and that a successful login returns you to bingo.
