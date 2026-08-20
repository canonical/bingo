---
myst:
  html_meta:
    "description lang=en": "How-to guides for operating and troubleshooting the bingo charm."
---

(how_to_index)=

# How-to guides

These guides assume you have basic familiarity with the bingo charm. They address the
operational surfaces exposed through Juju configuration options and actions — tuning how the
application handles requests and serves content, securing access through OIDC authentication and
credential rotation, and integrating with the wider Juju ecosystem for ingress and log
aggregation.

## Application configuration
<!--
Themes: base URL and routing, paste size limits, frontend asset delivery, log verbosity
Justification: shared operational concern — tuning the charm's `juju config` options that
  control how the deployed application behaves and serves content
User journey context: configuration phase, integration (ingress, log aggregation),
  ongoing maintenance and troubleshooting
Juju ecosystem scope: charm-specific (config options), cross-charm (ingress via traefik-k8s,
  logging via loki-k8s)
-->

- {ref}`Set the base URL <how_to_set_base_url>`
- {ref}`Limit paste size <how_to_limit_paste_size>`
- {ref}`Serve frontend assets <how_to_serve_frontend_assets>`
- {ref}`Configure logging <how_to_configure_logging>`

## Authentication and security
<!--
Themes: OIDC/OAuth integration, secret key lifecycle, session security
Justification: shared concern — securing access to the application and managing the
  credential and session lifecycle
User journey context: integration (identity provider), maintenance, security incident
  response
Juju ecosystem scope: cross-charm (oauth relation with identity provider), charm-specific
  (rotate-secret-key action)
-->

- {ref}`Configure OIDC login <how_to_configure_oidc_login>`
- {ref}`Rotate the secret key <how_to_rotate_secret_key>`

## Advanced operations
<!--
Themes: documentation contribution workflow
Justification: single-page topic without a shared peer — merged into fallback
User journey context: not tied to the deployment lifecycle; documentation process
Fallback: weaker thematic connection; narrative can be framed by the specific guide
-->

- {ref}`Contribute <how_to_contribute>`

```{toctree}
:hidden:
:maxdepth: 1

Set the base URL <set-base-url>
Limit paste size <limit-paste-size>
Configure logging <configure-logging>
Serve frontend assets <serve-frontend-assets>
Configure OIDC login <configure-oidc-login>
Rotate the secret key <rotate-secret-key>
Contribute <contribute>
```
