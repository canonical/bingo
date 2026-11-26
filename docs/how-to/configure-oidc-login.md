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

## Configure the username attribute

Bingo registers the callback path `/auth/callback` and requests the `openid`, `email`, and
`profile` scopes. These values are fixed in the charm's `paas-config.yaml` to match the application.

The `oauth-user-name-attribute` option defaults to `sub`. To select a different username claim
for the relation, run:

```
juju config bingo oauth-user-name-attribute=email
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
