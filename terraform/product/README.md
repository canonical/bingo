# bingo product Terraform module

This folder contains a self-contained [Terraform][Terraform] module that
deploys a working `bingo` model, including bundled dependency charms and all
relations. It is the "reusable building block" referenced by the bingo
Terraform module ticket: apply it and get a running bingo deployment without
reading the charm source.

Unlike the [base module](../README.md), this module:

- Bundles deployable backend charms for `postgresql`, `oauth`, and `ingress`
  (each optional, controlled by a `deploy_*` flag).
- Wires `tracing`, `logging`, `metrics-endpoint`, and `grafana-dashboard` only
  via externally-supplied Juju **offer URLs** — no charm is bundled for these,
  since they require pointing at existing observability infrastructure (e.g.
  a Canonical Observability Stack deployment).

Like the base module, it does **not** create the Juju model itself — you must
supply the UUID of an existing model via `model_uuid`.

## Module structure

- **main.tf** - Instantiates the [base module](../) and conditionally deploys
  `postgresql-k8s`, `oauth-external-idp-integrator`, and `traefik-k8s`, plus
  all `juju_integration` resources, into the given model.
- **variables.tf** - Target model UUID, bingo charm config, per-dependency
  `deploy_*` flags and config objects, and offer URLs for tracing/logging/
  metrics/grafana-dashboard.
- **outputs.tf** - Bingo app name/endpoints, and the names of any bundled
  dependency applications.
- **versions.tf** - Pins the required Terraform and `juju` provider versions.
- **tests/** - `terraform test` suite; requires a real Juju controller and
  Kubernetes cloud (intended for CI).

## Inputs

| Name | Type | Default | Description |
|---|---|---|---|
| `model_uuid` | `string` | *(required)* | UUID of the Juju model where the applications will be deployed. The model must already exist; this module does not create one. |
| `bingo` | `object` | see below | bingo charm settings: `app_name`, `channel`, `revision`, `base`, `config` (map(string)), `units`. |
| `deploy_postgresql` | `bool` | `true` | Deploy the bundled `postgresql-k8s` charm and integrate it. Set `false` to manage PostgreSQL integration yourself. |
| `postgresql` | `object` | see below | `postgresql-k8s` settings: `channel`, `revision`, `config`, `units`. Used only when `deploy_postgresql = true`. |
| `deploy_oauth` | `bool` | `true` | Deploy the bundled `oauth-external-idp-integrator` charm and integrate it. Set `false` to manage OAuth integration yourself. |
| `oauth` | `object` | see below | `oauth-external-idp-integrator` settings: `channel`, `revision`, `config`. **`config` must describe a real external IdP** (issuer URL, client ID/secret, etc.) — this module cannot provide that for you. Used only when `deploy_oauth = true`. |
| `deploy_ingress` | `bool` | `true` | Deploy the bundled `traefik-k8s` charm and integrate it. Set `false` to manage ingress yourself. |
| `traefik` | `object` | see below | `traefik-k8s` settings: `channel`, `revision`, `config`. Used only when `deploy_ingress = true`. |
| `tracing_offer_url` | `string` | `null` | Juju offer URL of an existing tracing backend (e.g. Charmed Tempo). `null` skips the tracing integration. |
| `logging_offer_url` | `string` | `null` | Juju offer URL of an existing Loki deployment. `null` skips the logging integration. |
| `metrics_offer_url` | `string` | `null` | Juju offer URL of an existing Prometheus deployment. `null` skips the metrics-endpoint integration. |
| `grafana_dashboard_offer_url` | `string` | `null` | Juju offer URL of an existing Grafana deployment. `null` skips the grafana-dashboard integration. |

## Outputs

| Name | Description |
|---|---|
| `bingo` | `{ app_name, requires, provides }` — bingo's application name and relation endpoint name maps. |
| `postgresql_app_name` | Name of the bundled PostgreSQL application, or `null` if `deploy_postgresql = false`. |
| `oauth_app_name` | Name of the bundled oauth-external-idp-integrator application, or `null` if `deploy_oauth = false`. |
| `ingress_app_name` | Name of the bundled traefik-k8s application, or `null` if `deploy_ingress = false`. |

## Required external inputs

Before you can `terraform apply` a fully working deployment you need:

1. An existing Juju model (`model_uuid`) on a cloud/controller with the
   required charms available.
2. If `deploy_oauth = true`: real identity provider details (issuer URL,
   client ID, client secret, and any other fields required by
   [`oauth-external-idp-integrator`](https://charmhub.io/oauth-external-idp-integrator))
   supplied via `var.oauth.config`.
3. If you want tracing, logging, metrics, or Grafana dashboards: the
   corresponding Juju offer URL(s) from an existing observability deployment
   (e.g. a Canonical Observability Stack model), supplied via
   `tracing_offer_url` / `logging_offer_url` / `metrics_offer_url` /
   `grafana_dashboard_offer_url`.

## Example usage

```hcl
resource "juju_model" "bingo" {
  name = "bingo-production"

  cloud {
    name = "my-k8s-cloud"
  }
}

module "bingo" {
  source = "git::https://github.com/canonical/bingo//terraform/product"

  model_uuid = juju_model.bingo.uuid

  bingo = {
    config = {
      "base-url" = "https://paste.example.com"
    }
  }

  oauth = {
    config = {
      issuer_url    = "https://login.example.com"
      client_id     = "bingo"
      client_secret = var.idp_client_secret
    }
  }

  tracing_offer_url           = "admin/cos.tempo-tracing"
  logging_offer_url           = "admin/cos.loki-logging"
  metrics_offer_url           = "admin/cos.prometheus-metrics-endpoint"
  grafana_dashboard_offer_url = "admin/cos.grafana-dashboards"
}
```

[Terraform]: https://developer.hashicorp.com/terraform
