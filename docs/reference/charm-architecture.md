---
myst:
  html_meta:
    "description lang=en": "Describes bingo's charm architecture: the workload container, environment variable conventions, and how Juju events are handled."
---

(reference_charm_architecture)=

# Charm architecture

bingo is a Go application providing paste creation, retrieval, and expiry - a self-hosted
pastebin service for Canonical. The charm is built with the
[`go-framework` Charmcraft extension](https://canonical.com/juju/docs/charmcraft/latest/reference/extensions/go-framework-extension/),
part of the {doc}`12-factor app support <12-factor:index>` provided by
Charmcraft/Rockcraft/`paas-charm`.

The general container/sidecar layout, Pebble usage, and `PaasCharm` base class shared by every
12-factor app charm are already documented centrally:

- {ref}`Charm architecture <12-factor:ref_charm_architecture>` - the sidecar pattern, the
  `app`/charm container split, and how OCI images are built with Rockcraft.
- {ref}`Juju events <12-factor:ref_juju_events>` - the full list of events any `paas-charm`-based
  charm may observe, and its default response to each.
- {ref}`How 12-factor principles are applied <12-factor:explanation_12_factor_principles_applied>`
  - why the framework works the way it does.

This page only covers what's specific to bingo's deployment of that framework.

## bingo container

bingo's workload container (Kubernetes container name `app`, per the `go-framework` extension) runs
the compiled `bingo` binary (built from `cmd/bingo/`) directly under Pebble. The process runs as
the `_daemon_` user with working directory `/app`, and listens on port `8080` by default (the
`app-port` framework configuration).

The binary reads its configuration from environment variables: charm configuration options (for
example `base-url`, `max-paste-size-bytes`, `log-level`, `web-dir`) are injected as `APP_*`, while
integration data is injected using each integration's own convention - for example
`POSTGRESQL_DB_CONNECT_STRING` for the database, and standard `OTEL_*` variables for tracing.

The image is defined in [`rockcraft.yaml`](https://github.com/canonical/bingo/blob/main/rockcraft.yaml)
at the repository root, using the
[`go-framework` Rockcraft extension](https://ubuntu.com/containers/rockcraft/docs/latest/reference/extensions/go-framework/)
with two customizations:
the main package lives at `cmd/bingo/` rather than the module root, and the production Vite build
of the React frontend is staged into the image under `/app/web/dist`, served by the binary as a
single-page application when the `web-dir` config option is set.

## Juju events

bingo's `charmcraft.yaml` declares `postgresql` (required), `tracing` (optional), and `oauth`
(optional) under `requires` (alongside the `logging`/`ingress`/`secret-storage` relations injected
automatically by the `go-framework` extension). Of all the integration-related events listed in
the {ref}`generic Juju events reference <12-factor:ref_juju_events>`, only the ones tied to those
relations are ever fired for bingo:

- PostgreSQL: `database_created`, `endpoints_changed`, `database_relation_broken`
- OAuth: `oauth_info_changed`, `oauth_info_removed`
- Tracing: `tracing_endpoint_changed`, `tracing_endpoint_removed`
- Ingress: `ingress_ready`, `ingress_revoked`
- `rotate_secret_key`

Events tied to the other integrations `paas-charm` supports (MySQL/MongoDB, Redis, Valkey, S3,
SAML, RabbitMQ, SMTP) never occur, since bingo's `charmcraft.yaml` doesn't declare those relations.

Two behaviors are specific to bingo, on top of the generic response described in the events
reference:

- On every `config_changed` event, after the inherited handler validates configuration and
  restarts the workload, `BingoCharm`'s own observer blocks the unit if `oauth-redirect-path` was
  changed away from its required fixed value (`/auth/callback`) - bingo's OIDC callback route is
  hardcoded and does not read that config option.
- The generic events reference describes "run pending migrations" as part of the standard
  response. bingo ships no `migrate`/`migrate.sh`/`migrate.py`/`manage.py` in its image, so
  `paas-charm`'s own migration runner never actually executes anything for bingo - the `bingo`
  binary instead applies its own migrations internally on every process start (see
  [`database.Migrate` in `cmd/bingo/main.go`](https://github.com/canonical/bingo/blob/main/cmd/bingo/main.go)).

## Charm code overview

[`charm/src/charm.py`](https://github.com/canonical/bingo/blob/main/charm/src/charm.py) defines
the `BingoCharm` class, which inherits from `paas_charm.go.Charm` (itself a `PaasCharm` subclass;
see the generic
[charm code overview](https://canonical.com/juju/docs/12-factor/latest/reference/charm-architecture/#charm-code-overview)
for how `PaasCharm.__init__` wires up event observers). `BingoCharm` adds two customizations on top
of the inherited behavior:

- It observes `config_changed` a second time (after the parent class's own handler) to block the
  unit if `oauth-redirect-path` has been changed away from its required value, as described above.
- It overrides the `_base_url` property so that the `base-url` config option, when set, takes
  priority over the ingress-derived URL that `paas_charm.go.Charm._base_url` would otherwise
  always return - letting operators control the externally-visible link text used in generated
  paste URLs.

See {ref}`Relation endpoints <reference_relation_endpoints>` for bingo's full list of relations,
and {ref}`Charm <juju:charm>` for more on the charm lifecycle in general.
