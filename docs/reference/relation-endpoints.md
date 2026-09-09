---
myst:
  html_meta:
    "description lang=en": "Reference documentation for all relation endpoints supported by the bingo charm."
---

(reference_relation_endpoints)=

# Relation endpoints

See [Integrations](https://charmhub.io/bingo/integrations).

<!-- vale Canonical.007-Headings-sentence-case = NO -->
<!-- The headings are relation endpoints, so they match the lowercase names in charmcraft.yaml -->

## grafana-dashboard

_Interface_: `grafana_dashboard`

_Supported charms_: [`grafana-k8s`](https://charmhub.io/grafana-k8s)

The `grafana-dashboard` relation ships a pre-built [Grafana](https://grafana.com/oss/grafana/)
dashboard for `bingo` so operators can
monitor the charm without building one from scratch. Once integrated, the dashboard appears in
Grafana's dashboard browser (`/dashboards`). Edits made in the Grafana UI are not persisted
across charm upgrades or when the charm is redeployed.

Example `grafana-dashboard` integrate command:

```
juju integrate bingo:grafana-dashboard grafana-k8s:grafana-dashboard
```

## ingress

_Interface_: `ingress`

_Supported charms_: [`traefik-k8s`](https://charmhub.io/traefik-k8s),
[`ingress-configurator`](https://charmhub.io/ingress-configurator)

The `ingress` relation exposes `bingo`'s HTTP interface outside the Kubernetes cluster and provides 
the public URL that should be configured as {ref}`base-url <how_to_set_base_url>`. This relation is
limited to a single application. `ingress-configurator` can be used instead of `traefik-k8s`
to route through [HAProxy](https://www.haproxy.org/) (`haproxy-route`) or
[Gateway API](https://gateway-api.sigs.k8s.io/) (`gateway-route`).

Example `ingress` integrate command:

```
juju integrate bingo:ingress traefik-k8s:ingress
```

To route through a shared HAProxy instance offered from another model instead, consume the
offer, deploy `ingress-configurator`, and integrate `bingo` → `ingress-configurator` → the
consumed offer:

```
juju consume <offer-url>
juju integrate bingo:ingress ingress-configurator:ingress
juju integrate ingress-configurator:haproxy-route <offer-name>:haproxy-route
```

## logging

_Interface_: `loki_push_api`

_Supported charms_: [`loki-k8s`](https://charmhub.io/loki-k8s)

The `logging` relation forwards `bingo`'s application logs to [Loki](https://grafana.com/oss/loki/)
for centralized log aggregation and querying through Grafana or the Loki API.

Example `logging` integrate command:

```
juju integrate bingo:logging loki-k8s:logging
```

## metrics-endpoint

<!-- vale Canonical.000-US-spellcheck = NO -->
<!-- prometheus_scrape is the name of the interface -->
_Interface_: `prometheus_scrape`
<!-- vale Canonical.000-US-spellcheck = YES -->

_Supported charms_: [`prometheus-k8s`](https://charmhub.io/prometheus-k8s)

The `metrics-endpoint` relation allows [Prometheus](https://prometheus.io/) to scrape the
`/metrics` endpoint exposed by `bingo`, enabling dashboards and alerting on request rates,
latencies, and other application metrics once the relation becomes active.

Example `metrics-endpoint` integrate command:

```
juju integrate bingo:metrics-endpoint prometheus-k8s:metrics-endpoint
```

## oauth

_Interface_: `oauth`

_Supported charms_: [`hydra`](https://charmhub.io/hydra)

The `oauth` relation lets `bingo` delegate authentication to an OpenID Connect provider so users
can log in with their existing identity provider credentials. The simplest way to provide an OIDC
provider is to deploy
[Hydra](https://charmhub.io/hydra) as part of the
[Canonical Identity Platform](https://canonical-identity.readthedocs-hosted.com/tutorial/canonical-identity-platform/).
This relation is optional and limited to a single application; see
{ref}`Configure OIDC login <how_to_configure_oidc_login>` for the related configuration
options.

Example `oauth` integrate command:

```
juju integrate bingo:oauth hydra:oauth
```

## postgresql

_Interface_: `postgresql_client`

_Supported charms_: [`postgresql-k8s`](https://charmhub.io/postgresql-k8s)

The `postgresql` relation is required by `bingo`. `bingo` stores pastes and, when OIDC is 
enabled, session data in a PostgreSQL database. The `bingo` charm will not start until this
relation is established.

Example `postgresql` integrate command:

```
juju integrate bingo:postgresql postgresql-k8s:database
```

## tracing

_Interface_: `tracing`

_Supported charms_: [`tempo-coordinator-k8s`](https://charmhub.io/tempo-coordinator-k8s)

The `tracing` relation sends distributed traces from `bingo` to [Tempo](https://grafana.com/oss/tempo/),
helping diagnose latency and errors across requests. This relation is optional and limited to
a single application.

Example `tracing` integrate command:

```
juju integrate bingo:tracing tempo-coordinator-k8s:tracing
```
