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

These guides let you configure your deployment for your particular use case and needs,
covering how the charm serves and routes requests and delivers content.

- {ref}`Set the base URL <how_to_set_base_url>`
- {ref}`Limit paste size <how_to_limit_paste_size>`
- {ref}`Serve frontend assets <how_to_serve_frontend_assets>`

## Authentication and security

The charm comes with built-in security features to control access to the application and
manage the credential and session lifecycle, from integrating with an identity provider to
rotating secrets.

- {ref}`Configure OIDC login <how_to_configure_oidc_login>`
- {ref}`Rotate the secret key <how_to_rotate_secret_key>`

## Maintenance and development

These guides support the ongoing upkeep of the charm and its documentation, from tuning
operational visibility to contributing to the project.

- {ref}`Configure logging <how_to_configure_logging>`
- {ref}`Contribute <how_to_contribute>`

```{toctree}
:hidden:
:maxdepth: 1

Set the base URL <set-base-url>
Limit paste size <limit-paste-size>
Serve frontend assets <serve-frontend-assets>
Configure OIDC login <configure-oidc-login>
Rotate the secret key <rotate-secret-key>
Configure logging <configure-logging>
Contribute <contribute>
```
