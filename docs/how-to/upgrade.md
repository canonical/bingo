---
myst:
  html_meta:
    "description lang=en": "Learn how to upgrade the bingo charm to the paas-charm 2.0, Ubuntu 26.04 LTS release."
---

(how_to_upgrade)=

# How to upgrade

Starting with charm revision 21, the bingo charm is built on `paas-charm` 2.0 and runs on the Ubuntu 26.04 LTS
base. The charm and its OCI image must be upgraded together.

## Keep the application secret key

The upgrade regenerates the application secret key, which signs out every user logged in through
OIDC. To keep existing sessions, or to control the key yourself, store it in a Juju secret and
pass it to the `app-secret-key` option:

```shell
juju add-secret bingo-secret-key value=<secret-string>
juju grant-secret bingo-secret-key bingo
juju config bingo app-secret-key=secret:<secret id>
```

## Refresh the charm

Refresh to the new charm, selecting the Ubuntu 26.04 LTS base and the new rock:

```shell
juju refresh bingo --channel <channel> --base ubuntu@26.04 \
  --resource app-image=<registry>/bingo:<version>
```

Check that the unit returns to `active` and that the `postgresql`, `ingress`, `oauth` and
observability integrations are still in place:

```shell
juju status bingo --relations
```
