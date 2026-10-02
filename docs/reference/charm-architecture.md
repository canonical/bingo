---
myst:
  html_meta:
    "description lang=en": "Describes bingo's charm architecture: the workload container, environment variable conventions, and how Juju events are handled."
---

(reference_charm_architecture)=

# Charm architecture

bingo is a Go application providing paste creation, retrieval, and expiry - a self-hosted
pastebin service for Canonical. The charm is built with the
{ref}`go-framework Charmcraft extension <charmcraft:go-framework-extension>`,
part of the {doc}`12-factor app support <12-factor:index>` provided by
Charmcraft, Rockcraft, and `paas-charm`.

The general container/sidecar layout, Pebble usage, and `PaasCharm` base class shared by every
12-factor app charm are already documented centrally:

- {ref}`Charm architecture <12-factor:ref_charm_architecture>` - the sidecar pattern, the
  `app`/charm container split, and how OCI images are built with Rockcraft.
- {ref}`Juju events <12-factor:ref_juju_events>` - the full list of events any `paas-charm`-based
  charm may observe, and its default response to each.
- {ref}`How 12-factor principles are applied <12-factor:explanation_12_factor_principles_applied>` -
  why the framework works the way it does.

This page only covers what's specific to bingo's usage of `go-framework`.

## bingo container

bingo's workload container (Kubernetes container name `app`, per the `go-framework` extension) runs
the compiled `bingo` binary (built from `cmd/bingo/`) directly under Pebble. The process runs as
the `_daemon_` user with working directory `/app`, and listens on port `8080` by default (the
`app-port` framework configuration).

The binary reads its configuration from environment variables: charm configuration options (for
example `base-url`, `max-paste-size-bytes`, `log-level`, `web-dir`) are injected as `APP_*`, while
integration data is injected using each integration's own convention. For example,
`POSTGRESQL_DB_CONNECT_STRING` is used for the database, and standard `OTEL_*` variables are used for tracing.

The image is defined in [`rockcraft.yaml`](https://github.com/canonical/bingo/blob/main/rockcraft.yaml)
at the repository root, using the
{ref}`go-framework Rockcraft extension <rockcraft:reference-go-framework>`
with two customizations:
the main package lives at `cmd/bingo/` rather than the module root, and the production Vite build
of the React frontend is staged into the image under `/app/web/dist`, served by the binary as a
single-page application when the `web-dir` configuration option is set.

(charm_architecture_code_overview)=

## Charm code overview

[`charm/src/charm.py`](https://github.com/canonical/bingo/blob/main/charm/src/charm.py) defines
the `BingoCharm` class, which inherits from `paas_charm.go.Charm` (itself a `PaasCharm` subclass;
see the generic {ref}`charm code overview <12-factor:ref_charm_architecture_code_overview>`
for how `PaasCharm.__init__` wires up event observers).

`BingoCharm` adds two customizations on top of the inherited behavior:

- It observes `config_changed` a second time (after the parent class's own handler) to block the
  unit if `oauth-redirect-path` has been changed away from its required fixed value
  (`/auth/callback`) - bingo's OIDC callback route is hardcoded and does not read that
  configuration option.
- It overrides the `_base_url` property so that the `base-url` configuration option, when set, takes
  priority over the ingress-derived URL that `paas_charm.go.Charm._base_url` would otherwise
  always return, letting operators control the externally-visible link text used in generated
  paste URLs.

## Juju events

bingo's [`charmcraft.yaml`](https://github.com/canonical/bingo/blob/main/charm/charmcraft.yaml)
declares `postgresql`, `tracing`, and `oauth` under `requires`, alongside the
`logging`/`ingress`/`secret-storage` relations injected automatically by the `go-framework`
extension. Of all the integration-related events listed in the {ref}`generic Juju events
reference <12-factor:ref_juju_events>`, only the ones tied to those relations are ever fired for
bingo:

- PostgreSQL (required): `database_created`, `endpoints_changed`, `database_relation_broken`
- OAuth (optional): `oauth_info_changed`, `oauth_info_removed`
- Tracing (optional): `tracing_endpoint_changed`, `tracing_endpoint_removed`
- Ingress (auto-injected): `ingress_ready`, `ingress_revoked`
- `rotate_secret_key`

Events tied to the other integrations `paas-charm` supports never occur, since bingo's
[`charmcraft.yaml`](https://github.com/canonical/bingo/blob/main/charm/charmcraft.yaml) doesn't
declare those relations.

On top of the generic response described in the events reference, two behaviors are specific to
bingo:

- Every `config_changed` event also validates `oauth-redirect-path`: if it has been changed away
  from its required fixed value, the unit is blocked (see {ref}`Charm code overview
  <charm_architecture_code_overview>` above).
- No event ever triggers `paas-charm`'s migration runner, since bingo ships no migrate script in
  its image - the `bingo` binary instead applies its own migrations internally on every process
  start (see
  `database.Migrate` in [`cmd/bingo/main.go`](https://github.com/canonical/bingo/blob/main/cmd/bingo/main.go)).

See {ref}`Relation endpoints <reference_relation_endpoints>` for bingo's full list of relations,
and {ref}`Charm <juju:charm>` for more on the charm lifecycle in general.
