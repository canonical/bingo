---
myst:
  html_meta:
    "description lang=en": "Learn how to rotate the bingo charm's shared application secret key."
---

(how_to_rotate_secret_key)=

# How to rotate the secret key

This guide provides instructions for rotating bingo's shared application secret key. This secret
is only used to sign and verify OIDC session cookies. It is not used for CSRF tokens or any other purpose.

## Prerequisites

Deploy the bingo charm.

```
juju deploy bingo
```

## Rotate the secret key

This charm provides a `rotate-secret-key` action to rotate the secret key. This is useful if a
security breach occurs or the secret key needs to be rotated as routine hygiene.

This action's effect depends on whether OIDC authentication is enabled (see
{ref}`how_to_configure_oidc_login`):

- **OIDC enabled**: existing session cookies were signed with the old secret and can no longer be
  verified after rotation, so all currently logged-in users are forced to log in again.
- **OIDC not enabled**: the secret is not currently used by any active code path, so rotating it
  has no user-visible effect.

Run the action against the leader unit:

```
juju run bingo/leader rotate-secret-key
```

```{terminal}
:output-only:

Running operation 1 with 1 task
  - task 2 on unit-bingo-0

Waiting for task 2...
status: success
```

## Verify

The `status: success` line in the action output above confirms the rotation completed.

If OIDC is enabled, confirm a previously authenticated session is no longer valid by refreshing
the browser session, or by reusing an old session cookie against bingo; you should be redirected
to log in again.
