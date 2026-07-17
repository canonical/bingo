# bingo

A Juju charm deploying and managing bingo on Kubernetes. bingo is a 12-Factor Go + React pastebin application that replaces paste.canonical.com, providing paste creation, retrieval, expiry, and optional OIDC authentication via the Canonical Identity Platform.

Like any Juju charm, this charm supports one-line deployment, configuration, integration, scaling, and more. For bingo, this includes:
* Paste creation and retrieval with configurable expiry
* Optional OIDC authentication via the Canonical Identity Platform
* Integration with PostgreSQL for persistent storage
* Ingress support via Traefik
* Observability integrations (Prometheus, Grafana, Loki)

## Get started

### Deploy

bingo requires a database for persistent storage, and it supports an ingress relation for public HTTP access. Deploy bingo and integrate it with the [PostgreSQL](https://charmhub.io/postgresql-k8s) and [Traefik](https://charmhub.io/traefik-k8s) charms:


```bash
juju add-model bingo
juju deploy bingo --resource app-image=<oci-image>
juju integrate bingo postgresql-k8s
juju integrate bingo traefik-k8s
```

### Basic operations

#### Configure the base URL

Set the public-facing URL for generated paste links:

```bash
juju config bingo base-url=https://paste.example.com
```

#### Configure paste size limit

```bash
juju config bingo max-paste-size-bytes=5242880
```

#### Rotate the session secret

If you suspect credential compromise, invalidate all existing sessions:

```bash
juju run bingo/0 rotate-secret-key
```

## Integrations

Required relations:
* `postgresql` is required for core app functionality (persistent storage).
* `ingress` is required only when exposing bingo externally (for public HTTP access via Traefik).

| Endpoint | Interface | Required | Purpose |
|---|---|---|---|
| `postgresql` | `postgresql_client` | Yes | Persistent paste storage |
| `ingress` | `ingress` | Required for external access | External HTTP access via Traefik |
| `logging` | `loki_push_api` | No | Log forwarding to Loki |
| `metrics-endpoint` | `prometheus_scrape` | No | Metrics scraping |
| `grafana-dashboard` | `grafana_dashboard` | No | Pre-built dashboards |
| `tracing` | `tracing` | No | Distributed tracing |

## Terraform module

A reusable [Terraform][terraform] module for deploying bingo via the
[Juju Terraform provider][juju-terraform-provider] is available in
[`terraform/`](terraform/README.md) (application only) and
[`terraform/product/`](terraform/product/README.md) (model + application +
bundled dependencies + relations — a full working deployment).

[terraform]: https://developer.hashicorp.com/terraform
[juju-terraform-provider]: https://registry.terraform.io/providers/juju/juju/latest/docs

## Project and community

* [Issues](https://github.com/canonical/bingo/issues)
* [Contributing](CONTRIBUTING.md)
* [Matrix](https://matrix.to/#/#charmhub-charmdev:ubuntu.com)

## Documentation

Our documentation is stored in the `docs` directory.
It is based on the Canonical starter pack
and hosted on [Read the Docs](https://canonical-bingo.readthedocs-hosted.com/latest/). In structuring,
the documentation employs the [Diátaxis](https://diataxis.fr/) approach.

You may open a pull request with your documentation changes, or you can
[file a bug](https://github.com/canonical/bingo/issues) to provide constructive feedback or suggestions.

To run the documentation locally before submitting your changes:

```bash
cd docs
make run
```

GitHub runs automatic checks on the documentation
to verify spelling, validate links and style guide compliance.

You can (and should) run the same checks locally:

```bash
make spelling
make linkcheck
make vale
make lint-md
```