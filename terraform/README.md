# bingo Terraform module

This folder contains a base [Terraform][Terraform] module for the `bingo`
charm.

The module uses the [Terraform Juju provider][Terraform Juju provider] to
model the charm deployment onto any Kubernetes environment managed by
[Juju][Juju]. It deploys only the `bingo` application — it does not create a
Juju model and does not wire any relations. For a self-contained deployment
(model + bingo + dependency charms + relations), use the
[product module](./product/README.md) instead.

## Module structure

- **main.tf** - Defines the `juju_application` resource for bingo.
- **variables.tf** - Inputs for customizing the deployment (charm channel,
  revision, config, constraints, units) and the target model.
- **outputs.tf** - Exposes the application name plus maps of relation
  endpoint names, so calling modules can build `juju_integration` resources
  without hardcoding endpoint names.
- **versions.tf** - Pins the required Terraform and `juju` provider versions.
- **tests/** - `terraform test` suite (`main.tftest.hcl` + a `setup/` helper
  module that creates an ephemeral model). Requires a real Juju controller and
  Kubernetes cloud; intended to run in CI, not locally without infrastructure.

## Inputs

| Name | Type | Default | Description |
|---|---|---|---|
| `app_name` | `string` | `"bingo"` | Name of the application in the Juju model. |
| `model_uuid` | `string` | *(required)* | UUID of an existing Juju model. This module does not create the model. |
| `base` | `string` | `"ubuntu@24.04"` | Operating system base for the charm. |
| `channel` | `string` | `"latest/edge"` | Charmhub channel to deploy from. |
| `revision` | `number` | `null` | Pin a specific charm revision; `null` uses the latest in `channel`. |
| `config` | `map(string)` | `{}` | Charm config, passed straight through. See [Config options](#config-options) below. |
| `constraints` | `string` | `""` | Juju constraints string for the application. |
| `units` | `number` | `1` | Number of units to deploy. |

## Config options

`config` keys map directly to the charm's config options (see
`charm/charmcraft.yaml` in this repository, or
https://charmhub.io/bingo/configurations once published):

| Key | Purpose |
|---|---|
| `base-url` | Public base URL for generated paste links, injected as `APP_BASE_URL`. |
| `max-paste-size-bytes` | Maximum paste content size in bytes. |
| `log-level` | Logging level: `debug`\|`info`\|`warn`\|`error`. |
| `web-dir` | Path to built frontend assets served as a SPA; empty disables static file serving. |
| `oauth-redirect-path` | OIDC callback redirect path appended to `base-url`. Must match the app's `/auth/callback` route. |
| `oauth-scopes` | Space-separated OAuth scopes requested from the IdP. Must include `openid`. |
| `oauth-user-name-attribute` | Claim used to identify the authenticated user from the IdP's userinfo response. |

## Outputs

| Name | Description |
|---|---|
| `app_name` | Name of the deployed bingo application. |
| `requires` | Map of bingo's `requires` relations to endpoint names: `postgresql`, `oauth`, `tracing`, `ingress`, `logging`. `postgresql` is required for the app to function; `oauth`, `tracing`, `ingress`, and `logging` are optional. |
| `provides` | Map of bingo's `provides` relations to endpoint names: `metrics_endpoint` (endpoint name `metrics-endpoint`), `grafana_dashboard` (endpoint name `grafana-dashboard`). All optional. |

## External inputs this module does not manage

This module deploys bingo only. For a working end-to-end deployment you must
separately provide (or use the [product module](./product/README.md), which
bundles some of these):

- A PostgreSQL database reachable over the `postgresql_client` interface
  (`postgresql` relation, **required**).
- An OIDC-compliant identity provider, integrated over the `oauth` interface
  (`oauth` relation, optional — only needed if OIDC login is enabled).
- A tracing backend (e.g. Charmed Tempo) reachable over the `tracing`
  interface (optional).
- An ingress provider (e.g. Traefik) reachable over the `ingress` interface
  (optional, needed for external HTTP access).
- A Loki-compatible log aggregator reachable over the `logging` (`loki_push_api`)
  interface (optional — bingo pushes logs to it, so no external input is
  required for the app to function without it).
- Prometheus metrics-endpoint and Grafana dashboard integrations are optional
  and provided by bingo (bingo is the data source; no external input needed).

## Using the bingo base module in higher-level modules

```hcl
resource "juju_model" "my_model" {
  name = "bingo"
  cloud {
    name = "my-k8s-cloud"
  }
}

module "bingo" {
  source     = "git::https://github.com/canonical/bingo//terraform"
  model_uuid = juju_model.my_model.uuid

  config = {
    "base-url" = "https://paste.example.com"
  }
}

resource "juju_integration" "bingo_postgresql" {
  model_uuid = juju_model.my_model.uuid

  application {
    name     = module.bingo.app_name
    endpoint = module.bingo.requires.postgresql
  }

  application {
    name     = "postgresql-k8s"
    endpoint = "database"
  }
}
```

[Terraform]: https://developer.hashicorp.com/terraform
[Terraform Juju provider]: https://registry.terraform.io/providers/juju/juju/latest
[Juju]: https://juju.is
