# Bingo Mono-repo Scaffolding Guide

This document describes how to scaffold the **bingo** mono-repo: a single-charm,
single-rock repository that packages a Go workload into a Juju 12-factor charm and
an OCI rock, using the **`charm-ci`** CI system.

## References

- **Go community project layout** — `cmd/` vs `internal/` conventions:
  <https://github.com/golang-standards/project-layout>
- **github-runner-operators** — the reference mono-repo this guide is modeled on
  (multi-charm, uses the new `charm-ci` workflows):
  <https://github.com/canonical/github-runner-operators>
- **platform-engineering-charm-template** — the canonical single-charm template
  (note: it still uses the **old** `operator-workflows` CI, not `charm-ci`):
  <https://github.com/canonical/platform-engineering-charm-template>
- **charm-ci / opcli** — the new build/test/publish tooling and reusable workflows:
  <https://github.com/canonical/charm-ci>

## Background: charm-ci vs operator-workflows

The bingo repo must use the **new `charm-ci` system**, not the older
`operator-workflows` that the `platform-engineering-charm-template` still ships.

- **Old** (`platform-engineering-charm-template`): monolithic reusable workflows
  under `canonical/operator-workflows/.github/workflows/*` that manage build, test,
  and publish inline.
- **New** (`github-runner-operators`): `canonical/charm-ci/.github/workflows/*`,
  driven by a CLI tool called **`opcli`** and three declarative files:

  | File | Purpose |
  | --- | --- |
  | `artifacts.yaml` | Declares what to build: rocks, charms, and resource bindings. |
  | `spread.yaml` | Declares how to test: backends and integration suites. |
  | `concierge.yaml` | Environment provisioning (Juju channel, MicroK8s, LXD). |

  The three reusable `charm-ci` workflows:
  - `build-artifacts.yml` — builds rocks + charms in parallel → `artifacts.build.yaml`
  - `integration-test.yml` — downloads artifacts, generates the spread matrix, runs tests
  - `publish-artifacts.yml` — publishes validated artifacts to CharmHub, cuts GitHub Releases

Because the template repo predates `charm-ci`, the bingo repo will **diverge from the
template specifically in the CI files** — copy the workflow + `artifacts.yaml` +
`spread.yaml` pattern from `github-runner-operators` instead.

## Target project structure

`github-runner-operators` nests charms under `charms/<name>/` because it has **four**
charms. Bingo has **one** charm, so the charm files are flattened to the repo root
(matching the `platform-engineering-charm-template` layout).

```
bingo-operators/
│
├── artifacts.yaml              # charm-ci build manifest (1 rock + 1 charm)
├── spread.yaml                 # charm-ci test orchestration (1 integration suite)
├── concierge.yaml              # env provisioning (Juju, MicroK8s, LXD)
├── bingo-rockcraft.yaml        # single rock definition
├── build-bingo-rock.sh         # convenience build script
│
├── go.mod                      # Go module definition
├── go.sum
│
├── cmd/                        # Go executable entry points (thin main.go)
│   └── bingo/
│       ├── main.go             # app entry point
│       └── main_test.go
│
├── internal/                   # Go business logic (private to module)
│   ├── server/
│   │   ├── server.go
│   │   └── server_test.go
│   ├── database/
│   │   ├── database.go
│   │   └── database_test.go
│   └── ...                     # other domain packages
│
├── charmcraft.yaml             # single charm (go-framework extension)
├── pyproject.toml
├── requirements.txt
├── tox.toml                    # per-charm: fmt, lint, complexity, static, unit, coverage
│
├── src/                        # charm Python source
│   └── charm.py                # BingoCharm(paas_charm.go.Charm)
│
├── tests/
│   ├── unit/                   # Scenario-based unit tests
│   │   └── test_charm.py
│   └── integration/            # shared integration tests
│       └── test_charm.py
│
├── terraform/                  # Terraform module for the charm
│   └── ...
│
├── docs/                       # Diátaxis docs (optional)
│
├── AGENTS.md
├── CONTRIBUTING.md
├── README.md
├── LICENSE
│
└── .github/
    └── workflows/
        ├── charms_lint_and_unit.yaml   # runs tox -c tox.toml
        ├── charms_integration.yaml     # delegates to charm-ci integration-test.yml
        ├── publish_charms.yml          # delegates to charm-ci publish-artifacts.yml
        └── internal_tests.yaml         # Go: go test ./... with coverage gate
```

## Where the Go source code goes

The Go workload follows the
[Go community project layout](https://github.com/golang-standards/project-layout),
exactly as `github-runner-operators` does (`cmd/planner/main.go` +
`internal/planner/`):

| Path | Contents |
| --- | --- |
| `cmd/bingo/main.go` | **Entry point** — a thin `main.go` that wires up and starts the app. Keep it minimal (~10 lines in the reference repo). |
| `internal/` | **All business logic** — `server/`, `database/`, and other domain packages. Private to the module (Go enforces that `internal/` is unimportable from outside). |

- The rock (`bingo-rockcraft.yaml`) builds the binary from `cmd/bingo`.
- The charm (`charmcraft.yaml`, via the `go-framework` extension) packages that rock
  as its `app-image` OCI resource.
- The rock↔charm resource binding is declared in `artifacts.yaml`.

A single-file app could place `main.go` at the repo root, but `cmd/bingo/` is the
idiomatic choice here because the repo contains **both** Go workload code and Python
charm code — the `cmd/` + `internal/` split cleanly separates the two.

## Key scaffolding file: `artifacts.yaml`

Single rock + single charm, with the OCI resource binding:

```yaml
version: 1
rocks:
  - name: bingo
    rockcraft-yaml: bingo-rockcraft.yaml
    platforms:
      - arch: amd64
charms:
  - name: bingo
    charmcraft-yaml: charmcraft.yaml
    channel: latest/edge
    resources:
      app-image:
        type: oci-image
        rock: bingo
    platforms:
      - arch: amd64
snaps: []
```

## What to copy from `github-runner-operators`

| Component | Reference path in github-runner-operators | Adaptation for bingo |
| --- | --- | --- |
| Integration CI | `.github/workflows/charms_integration.yaml` | Copy as-is (already delegates to `charm-ci`). |
| Publish CI | `.github/workflows/publish_charms.yml` | Copy as-is; set `CHARMHUB_TOKEN` secret. |
| Lint/unit CI | `.github/workflows/charms_lint_and_unit.yaml` | Drop the charm matrix; point at the single charm dir. |
| Go tests CI | `.github/workflows/internal_tests.yaml` | Keep; adjust service containers to bingo's needs. |
| Build manifest | `artifacts.yaml` | Reduce to 1 rock + 1 charm (see above). |
| Test orchestration | `spread.yaml` | Reduce to a single integration suite. |
| Env provisioning | `concierge.yaml` | Reusable nearly as-is for K8s charms. |
| Charm skeleton | `charms/garm/` (a `paas_charm.go.Charm`) | Use as the template for the bingo charm. |
| Per-charm tox | `charms/garm/tox.toml` | Copy verbatim (fmt, lint, complexity, static, unit, coverage). |

## Conventions to preserve

- **12-factor `go-framework` charm**: a thin Python charm layer wraps the Go rock.
  Do **not** hand-author a Pebble layer or a second `_reconcile` method — the
  `paas_charm` base class owns reconciliation. Override framework hooks (e.g.
  `restart()` / `_create_app()`) and call `super()` when behaviour must be injected.
- **Gates**: ≥ 85% coverage on internal Go packages; cyclomatic complexity < 10 per
  function.
- **Unit tests**: use the **Scenario** framework (`scenario.Context` / `State`), not
  `Harness`.
- **Go**: standard library first; table-driven tests; keep functions under the
  complexity gate.
```
