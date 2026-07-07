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

| Endpoint | Interface | Purpose |
|---|---|---|
| `postgresql` | `postgresql_client` | Persistent paste storage |
| `ingress` | `ingress` | External HTTP access via Traefik |
| `logging` | `loki_push_api` | Log forwarding to Loki |
| `metrics-endpoint` | `prometheus_scrape` | Metrics scraping |
| `grafana-dashboard` | `grafana_dashboard` | Pre-built dashboards |
| `tracing` | `tracing` | Distributed tracing (optional) |

## Project and community

* [Issues](https://github.com/canonical/bingo/issues)
* [Contributing](CONTRIBUTING.md)
* [Matrix](https://matrix.to/#/#charmhub-charmdev:ubuntu.com)
