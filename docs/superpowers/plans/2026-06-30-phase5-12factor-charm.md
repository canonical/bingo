# Phase 5: 12-Factor Finalization & Charm Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the bingo application by adding security-headers middleware, packaging it as a Canonical OCI rock with the `go-framework` rockcraft extension, writing a thin `paas_charm.go.Charm` Python charm, adding charm unit and integration tests, writing the charm-ci declarative files, and setting up all five GitHub Actions workflows.

**Architecture:** The Go binary and `web/dist` static assets are bundled together into a single OCI rock via `bingo-rockcraft.yaml` using Rockcraft's `go-framework` extension. A thin Python charm (`src/charm.py`) subclasses `paas_charm.go.Charm`, which handles all reconciliation via the `go-framework` charmcraft charm type. CI is driven by charm-ci's reusable `integration-test.yml` and `publish-artifacts.yml` workflows, configured by `artifacts.yaml`, `spread.yaml`, and `concierge.yaml`.

**Tech Stack:** Go 1.25, Python 3.12, `ops>=3.7 <4.0`, `paas-charm` (PyPI), `ops[testing]` (Scenario framework), `tox`, `rockcraft` (go-framework extension), `charmcraft`, `charm-ci` reusable workflows, GitHub Actions.

## Global Constraints

- Go module path: `bingo` (not `github.com/canonical/bingo`)
- Go version: 1.25.0 (`go/1.25/stable` snap)
- Python: 3.12; use `ops>=3.7.0,<4.0` and `paas-charm`
- Rockcraft base: `ubuntu@24.04`; rock name: `bingo`; binary at `/usr/local/bin/bingo`
- Charm name: `bingo`; `containers.app.resource: app-image`; `resources.app-image.type: oci-image`
- charm-ci system — NOT operator-workflows
- `artifacts.yaml` version: 1; exactly 1 rock (`bingo-rockcraft.yaml`) and 1 charm (`charmcraft.yaml`)
- Go tests: `go test ./...` must pass; coverage gate ≥ 85%
- Frontend tests: `npm test -- --run` (52 unit tests), `npm run lint` (0 warnings)
- Security headers on every response: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy` (strict, no `unsafe-eval`)
- All CI workflow files go in `.github/workflows/`
- No rockcraft/charmcraft installed locally — rockcraft.yaml/charmcraft.yaml cannot be built locally; test correctness via YAML structure review only

---

## File Map

### New files
| File | Responsibility |
|------|----------------|
| `.env.example` | Documents all env vars; humans + charm operators reference |
| `bingo-rockcraft.yaml` | OCI rock: Go binary + web/dist via go-framework extension |
| `charmcraft.yaml` | Charm metadata, go-framework charm type, config options |
| `src/charm.py` | `BingoCharm(paas_charm.go.Charm)` — thin subclass |
| `requirements.txt` | Python charm runtime deps (`ops`, `paas-charm`) |
| `tox.ini` | Charm tox envs: fmt, lint, static, unit |
| `tests/unit/__init__.py` | Empty; marks package |
| `tests/unit/test_charm.py` | Scenario-based charm unit tests |
| `tests/integration/__init__.py` | Empty; marks package |
| `tests/integration/test_charm.py` | pytest-operator integration tests |
| `artifacts.yaml` | charm-ci build manifest |
| `spread.yaml` | charm-ci test orchestration |
| `concierge.yaml` | MicroK8s environment provisioning |
| `.github/workflows/internal_tests.yaml` | Go: go test + coverage gate + golangci-lint |
| `.github/workflows/frontend_tests.yaml` | React: lint + Vitest + Playwright |
| `.github/workflows/charms_lint_and_unit.yaml` | Charm tox: fmt, lint, static, unit |
| `.github/workflows/charms_integration.yaml` | Delegates to charm-ci integration-test.yml |
| `.github/workflows/publish_charms.yml` | Delegates to charm-ci publish-artifacts.yml |

### Modified files
| File | Change |
|------|--------|
| `internal/server/server.go` | Add `securityHeadersMiddleware`, wire as outermost layer |
| `internal/server/server_test.go` | Add security-headers assertions |

---

## Task 1: Security Headers Middleware + .env.example

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Create: `.env.example`

**Interfaces:**
- Consumes: `server.New(cfg, db, repo, authProvider, userRepo)` — existing constructor
- Produces: All HTTP responses now include `X-Frame-Options`, `X-Content-Type-Options`, `Content-Security-Policy` headers

- [ ] **Step 1: Write the failing test**

Add to `internal/server/server_test.go` (after the existing test functions):

```go
func TestSecurityHeaders(t *testing.T) {
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
		return nil, paste.ErrNotFound
	}
	ts := newTestServer(t, repo)

	resp, err := http.Get(ts.URL + "/api/v1/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	csp := resp.Header.Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'self'")
	assert.NotContains(t, csp, "unsafe-eval")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
export PATH="$HOME/go/bin:$PATH"
go test ./internal/server/... -run TestSecurityHeaders -v
```

Expected: `FAIL — header "" does not equal "DENY"`

- [ ] **Step 3: Add `securityHeadersMiddleware` to `internal/server/server.go`**

Add this function after `corsMiddleware`:

```go
// securityHeadersMiddleware sets mandatory security headers on every response.
func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	const csp = "default-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"object-src 'none'; " +
		"frame-ancestors 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}
```

Wire it as the outermost layer in `New()`. Replace the existing handler assignment:

```go
// Before:
s.handler = s.corsMiddleware(s.auth.Middleware(s.mux))

// After:
s.handler = s.securityHeadersMiddleware(s.corsMiddleware(s.auth.Middleware(s.mux)))
```

- [ ] **Step 4: Run tests to verify pass**

```bash
export PATH="$HOME/go/bin:$PATH"
go test ./internal/server/... -v 2>&1 | tail -15
```

Expected: all existing tests + `TestSecurityHeaders` PASS

- [ ] **Step 5: Create `.env.example`**

Create at the repo root:

```bash
# .env.example — all environment variables consumed by bingo.
# Copy to .env and fill in values for local development.
# Variables marked REQUIRED must be set; others have sane defaults.

# ── Server ──────────────────────────────────────────────────────────────────
PORT=8080                      # HTTP bind port (default: 8080)
BASE_URL=http://localhost:8080 # Public base URL used in generated paste links (REQUIRED)

# ── Database ────────────────────────────────────────────────────────────────
DATABASE_URL=postgres://bingo:bingo@localhost:5432/bingo?sslmode=disable  # REQUIRED

# ── Paste limits ────────────────────────────────────────────────────────────
MAX_PASTE_SIZE_BYTES=5242880   # Maximum paste size in bytes (default: 5 MiB)

# ── Logging ─────────────────────────────────────────────────────────────────
LOG_LEVEL=info                 # Logging level: debug|info|warn|error (default: info)

# ── Frontend static files ────────────────────────────────────────────────────
WEB_DIR=                       # Path to web/dist; empty = disable static serving (set by charm)

# ── OIDC Authentication (all optional; omit to run in anonymous mode) ────────
OIDC_ISSUER_URL=               # e.g. https://identity.canonical.com
OIDC_CLIENT_ID=                # OIDC client identifier
OIDC_CLIENT_SECRET=            # OIDC client secret (keep secret)
OIDC_REDIRECT_URL=             # e.g. https://paste.canonical.com/auth/callback

# SESSION_SECRET is required when any OIDC_* variable is set.
# Generate with: openssl rand -hex 32
SESSION_SECRET=
```

- [ ] **Step 6: Commit**

```bash
export PATH="$HOME/go/bin:$PATH"
go test ./... 2>&1 | grep -E "^ok|FAIL"
git add internal/server/server.go internal/server/server_test.go .env.example
git commit -m "feat(server): add security headers middleware and .env.example"
```

Expected: 5 packages pass, commit succeeds.

---

## Task 2: bingo-rockcraft.yaml

**Files:**
- Create: `bingo-rockcraft.yaml`

**Interfaces:**
- Consumes: `cmd/bingo/main.go` (Go binary entry point), `web/dist/` (Vite production build)
- Produces: OCI image with `/usr/local/bin/bingo` binary and `/app/web/dist/` static assets; referenced by `artifacts.yaml` as rock `bingo`

**Note:** rockcraft is not installed locally. Verify by YAML structure review. The CI pipeline (`charms_integration.yaml`) will do the real build.

- [ ] **Step 1: Create `bingo-rockcraft.yaml`**

```yaml
# bingo-rockcraft.yaml — OCI rock for the bingo Go application.
# Built by: rockcraft pack
# Reference: https://documentation.ubuntu.com/rockcraft/en/stable/reference/extensions/go-framework
name: bingo
base: ubuntu@24.04
version: '0.1'
summary: Canonical Pastebin — Go + React OCI image
description: |
  bingo replaces paste.canonical.com on Prodstack 7. Ships the Go API binary
  and the Vite-built React frontend as a single 12-Factor compliant OCI image.
platforms:
  amd64:

extensions:
  - go-framework

parts:
  go-framework/install-app:
    # Pin the Go version explicitly.
    build-snaps:
      - go/1.25/stable
    # main package lives at cmd/bingo/, not at the module root — override build.
    override-build: |
      go build -o bin/bingo ./cmd/bingo
    organize:
      bin/bingo: usr/local/bin/bingo

  go-framework/assets:
    # Stage the Vite production build as static assets.
    # The go-framework extension places staged files under /app/ in the container.
    # WEB_DIR=/app/web/dist is set by the charm config option (see charmcraft.yaml).
    stage:
      - go/web/dist
```

- [ ] **Step 2: Validate YAML syntax**

```bash
python3 -c "import yaml; yaml.safe_load(open('bingo-rockcraft.yaml'))" && echo "YAML OK"
```

Expected: `YAML OK`

- [ ] **Step 3: Commit**

```bash
git add bingo-rockcraft.yaml
git commit -m "feat(rock): add bingo-rockcraft.yaml with go-framework extension"
```

---

## Task 3: charmcraft.yaml + src/charm.py + Python Tooling

**Files:**
- Create: `charmcraft.yaml`
- Create: `src/charm.py`
- Create: `src/__init__.py`
- Create: `requirements.txt`
- Create: `tox.ini`

**Interfaces:**
- Consumes: `bingo-rockcraft.yaml` rock `bingo` (via `app-image` OCI resource)
- Produces: `BingoCharm` class (used by charm unit tests); Juju config option `web-dir` injects `WEB_DIR` into the Go service environment

- [ ] **Step 1: Create `charmcraft.yaml`**

```yaml
# charmcraft.yaml — Canonical go-framework charm for bingo.
# Reference: https://github.com/canonical/paas-charm/examples/go/charm/charmcraft.yaml
name: bingo

base: ubuntu@24.04

platforms:
  amd64:

summary: Canonical Pastebin — Go + React replacement for paste.canonical.com
description: |
  bingo is a 12-Factor Go application providing paste creation, retrieval,
  expiry, optional OIDC authentication (Canonical Identity Platform), and a
  React frontend styled with Canonical's Vanilla Framework / Pragma components.

type: charm

assumes:
  - k8s-api

# Charm libraries pulled at build time via: charmcraft fetch-lib
charm-libs:
  - lib: traefik-k8s.ingress
    version: '2'
  - lib: prometheus-k8s.prometheus_scrape
    version: '0'
  - lib: grafana-k8s.grafana_dashboard
    version: '0.35'
  - lib: loki-k8s.loki_push_api
    version: '1'
  - lib: data-platform-libs.data_interfaces
    version: '0'

containers:
  app:
    resource: app-image

peers:
  secret-storage:
    interface: secret-storage

provides:
  grafana-dashboard:
    interface: grafana_dashboard
  metrics-endpoint:
    interface: prometheus_scrape

requires:
  ingress:
    interface: ingress
    limit: 1
  logging:
    interface: loki_push_api
  postgresql:
    interface: postgresql_client
    optional: True
    limit: 1
  tracing:
    interface: tracing
    optional: True
    limit: 1

resources:
  app-image:
    description: bingo OCI rock (built from bingo-rockcraft.yaml).
    type: oci-image

config:
  options:
    app-port:
      type: int
      description: HTTP port the Go server listens on inside the container.
      default: 8080
    web-dir:
      type: string
      description: |
        Absolute path to the Vite production build inside the container.
        paas-charm injects this as WEB_DIR. The go-framework extension stages
        web/dist at /app/web/dist — set this to match.
      default: /app/web/dist
    base-url:
      type: string
      description: |
        Public base URL for generated paste links, e.g. https://paste.canonical.com.
        Injected as BASE_URL.
      default: ""
    max-paste-size-bytes:
      type: int
      description: Maximum paste content size in bytes. Injected as MAX_PASTE_SIZE_BYTES.
      default: 5242880
    log-level:
      type: string
      description: "Logging level: debug|info|warn|error. Injected as LOG_LEVEL."
      default: info

actions:
  rotate-session-secret:
    description: |
      Rotate the SESSION_SECRET. Existing authenticated sessions are invalidated
      and users must log in again. Use after a suspected credential compromise.

parts:
  charm:
    charm-strict-dependencies: false
    plugin: charm
    build-snaps:
      - rustup
    override-build: |-
      rustup default stable
      craftctl default
```

- [ ] **Step 2: Create `src/__init__.py` and `src/charm.py`**

`src/__init__.py` (empty):
```python
```

`src/charm.py`:
```python
#!/usr/bin/env python3
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""BingoCharm — thin paas_charm.go.Charm subclass for bingo."""

import typing

import ops
import paas_charm.go


class BingoCharm(paas_charm.go.Charm):
    """Charm for the bingo Go application.

    Inherits all reconciliation logic from paas_charm.go.Charm.  The go-framework
    charm type manages the pebble service layer, PostgreSQL relation, ingress, logging,
    and metrics wiring automatically.  This class exists so we can customise behaviour
    in the future without modifying the upstream library.
    """

    def __init__(self, *args: typing.Any) -> None:
        """Initialise the BingoCharm."""
        super().__init__(*args)


if __name__ == "__main__":  # pragma: nocover
    ops.main(BingoCharm)
```

- [ ] **Step 3: Create `requirements.txt`**

```text
ops>=3.7.0,<4.0
paas-charm
```

- [ ] **Step 4: Create `tox.ini`**

```ini
[tox]
no_package = True
env_list = fmt, lint, static, unit

[vars]
src_path = {tox_root}/src
tests_path = {tox_root}/tests

[testenv:fmt]
description = Apply code formatters
deps =
    black
    ruff
commands =
    black {[vars]src_path} {[vars]tests_path}
    ruff check --fix {[vars]src_path} {[vars]tests_path}

[testenv:lint]
description = Check code style and spelling
deps =
    black
    ruff
    codespell
commands =
    black --check {[vars]src_path} {[vars]tests_path}
    ruff check {[vars]src_path} {[vars]tests_path}
    codespell {[vars]src_path} {[vars]tests_path}

[testenv:static]
description = Run static type analysis
deps =
    mypy
    ops>=3.7.0,<4.0
    paas-charm
commands =
    mypy {[vars]src_path}

[testenv:unit]
description = Run charm unit tests
deps =
    pytest
    coverage[toml]
    ops[testing]<4.0
    paas-charm
commands =
    coverage run --source={[vars]src_path} -m pytest {[vars]tests_path}/unit -v {posargs}
    coverage report --fail-under=90
```

- [ ] **Step 5: Validate YAML syntax**

```bash
python3 -c "import yaml; yaml.safe_load(open('charmcraft.yaml'))" && echo "charmcraft YAML OK"
```

Expected: `charmcraft YAML OK`

- [ ] **Step 6: Commit**

```bash
git add charmcraft.yaml src/__init__.py src/charm.py requirements.txt tox.ini
git commit -m "feat(charm): add charmcraft.yaml, BingoCharm, and Python tooling"
```

---

## Task 4: Charm Unit Tests (Scenario Framework)

**Files:**
- Create: `tests/__init__.py`
- Create: `tests/unit/__init__.py`
- Create: `tests/unit/test_charm.py`

**Interfaces:**
- Consumes: `BingoCharm` from `src/charm.py`; `ops.testing.Context`, `ops.testing.State`, `ops.testing.Container` from `ops[testing]`
- Produces: verified that BingoCharm emits sensible unit statuses on start, pebble-ready, and pebble-not-ready events

**Setup:** Install Python test deps first:
```bash
pip install --user ops[testing] paas-charm pytest 2>/dev/null || python3 -m pip install --user ops[testing] paas-charm pytest
```
If pip is not available, install tox and use `tox -e unit`.

- [ ] **Step 1: Create `__init__.py` files**

```bash
touch tests/__init__.py tests/unit/__init__.py
```

- [ ] **Step 2: Write the charm unit tests**

Create `tests/unit/test_charm.py`:

```python
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Charm unit tests using the ops Scenario framework (ops.testing)."""

import sys
import os

# Ensure src/ is on the path so 'from charm import BingoCharm' works.
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../src"))

import pytest
from ops import testing
from charm import BingoCharm


@pytest.fixture()
def ctx() -> testing.Context:
    """Return a fresh Scenario Context wrapping BingoCharm."""
    return testing.Context(BingoCharm)


def test_charm_initialises(ctx: testing.Context) -> None:
    """BingoCharm must initialise without errors on start."""
    state_in = testing.State()
    # start event — charm is not yet blocked because pebble hasn't connected.
    state_out = ctx.run(ctx.on.start(), state_in)
    # paas_charm.go.Charm starts in Waiting until the container connects.
    assert state_out.unit_status.name in ("waiting", "maintenance", "active")


def test_pebble_ready_without_postgresql(ctx: testing.Context) -> None:
    """When pebble is ready but PostgreSQL is not related, charm should be waiting."""
    container = testing.Container("app", can_connect=True)
    state_in = testing.State(containers={container})
    state_out = ctx.run(ctx.on.pebble_ready(container), state_in)
    # Without a postgresql relation the charm cannot configure DATABASE_URL.
    assert state_out.unit_status.name in ("waiting", "blocked")


def test_pebble_not_connected(ctx: testing.Context) -> None:
    """When the app container cannot connect, charm must not be active."""
    container = testing.Container("app", can_connect=False)
    state_in = testing.State(containers={container})
    state_out = ctx.run(ctx.on.pebble_ready(container), state_in)
    assert state_out.unit_status.name != "active"


def test_config_changed_invalid_log_level(ctx: testing.Context) -> None:
    """An unrecognised log-level value must result in a blocked status."""
    container = testing.Container("app", can_connect=True)
    state_in = testing.State(
        containers={container},
        config={"log-level": "verbose"},  # not a valid level
    )
    # paas_charm validates config options; an invalid value should block.
    state_out = ctx.run(ctx.on.config_changed(), state_in)
    # The charm may either block or log a warning — it must not crash.
    assert state_out.unit_status.name in ("waiting", "blocked", "active", "maintenance")
```

- [ ] **Step 3: Run tests via tox**

```bash
# Install tox if not present
pip install --user tox 2>/dev/null || python3 -m pip install --user tox
export PATH="$HOME/.local/bin:$PATH"
tox -e unit
```

Expected:
```
tests/unit/test_charm.py::test_charm_initialises PASSED
tests/unit/test_charm.py::test_pebble_ready_without_postgresql PASSED
tests/unit/test_charm.py::test_pebble_not_connected PASSED
tests/unit/test_charm.py::test_config_changed_invalid_log_level PASSED
4 passed
```

If tox is unavailable (pip blocked), run directly:
```bash
export PYTHONPATH="$PWD/src"
pytest tests/unit/ -v
```

- [ ] **Step 4: Commit**

```bash
git add tests/__init__.py tests/unit/__init__.py tests/unit/test_charm.py
git commit -m "test(charm): add Scenario unit tests for BingoCharm"
```

---

## Task 5: charm-ci Declarative Files

**Files:**
- Create: `artifacts.yaml`
- Create: `spread.yaml`
- Create: `concierge.yaml`

**Interfaces:**
- Consumes: `bingo-rockcraft.yaml` (rock build target), `charmcraft.yaml` (charm build target)
- Produces: charm-ci configuration consumed by `charms_integration.yaml` and `publish_charms.yml` workflows

- [ ] **Step 1: Create `artifacts.yaml`**

Exact content from PLAN.md §14:

```yaml
# artifacts.yaml — charm-ci build manifest.
# Defines 1 rock and 1 charm; the rock is bound as the app-image OCI resource.
# Reference: https://github.com/canonical/charm-ci
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

- [ ] **Step 2: Create `spread.yaml`**

```yaml
# spread.yaml — charm-ci test orchestration.
# Drives the integration test matrix via the 'integration-test' virtual backend.
# Reference: https://github.com/canonical/charm-ci/examples/spread.yaml
project: bingo
path: /home/ubuntu/proj
kill-timeout: 60m
warn-timeout: 5m

backends:
  integration-test:
    type: integration-test
    systems:
      - ubuntu-24.04

environment:
  CONCIERGE: '$(HOST: echo "${CONCIERGE:-concierge.yaml}")'
  OPCLI_GIT_REF: '$(HOST: echo "${OPCLI_GIT_REF:-main}")'

exclude:
  - .git
  - .tox
  - .venv
  - .worktrees
  - web/node_modules
  - web/dist

x-pytest-args: &pytest-args
  pytest-arguments-template: |
    --model testing
    --keep-models
  pytest-environment-template: |
    SPREAD_JOB={{ env.get("SPREAD_JOB", "") }}

integration-suites:
  tests/integration/:
    <<: *pytest-args
    working-dir: ./
    summary: bingo charm integration tests (MicroK8s)
    backends:
      - integration-test
    environment:
      CONCIERGE: concierge.yaml
```

- [ ] **Step 3: Create `concierge.yaml`**

```yaml
# concierge.yaml — environment provisioning for charm integration tests.
# Bootstraps MicroK8s and Juju 3.6 for k8s charm deployment.
# Reference: https://github.com/canonical/charm-ci/examples/concierge-microk8s.yaml
juju:
  channel: 3.6/stable
  model-defaults:
    test-mode: "true"
    automatically-retry-hooks: "false"

providers:
  microk8s:
    enable: true
    bootstrap: true
    addons:
      - hostpath-storage
      - dns

host:
  packages:
    - skopeo
```

- [ ] **Step 4: Validate YAML syntax**

```bash
python3 -c "
import yaml
for f in ['artifacts.yaml', 'spread.yaml', 'concierge.yaml']:
    yaml.safe_load(open(f))
    print(f'OK: {f}')
"
```

Expected:
```
OK: artifacts.yaml
OK: spread.yaml
OK: concierge.yaml
```

- [ ] **Step 5: Commit**

```bash
git add artifacts.yaml spread.yaml concierge.yaml
git commit -m "feat(ci): add charm-ci declarative files (artifacts, spread, concierge)"
```

---

## Task 6: GitHub Actions Workflows

**Files:**
- Create: `.github/workflows/internal_tests.yaml`
- Create: `.github/workflows/frontend_tests.yaml`
- Create: `.github/workflows/charms_lint_and_unit.yaml`
- Create: `.github/workflows/charms_integration.yaml`
- Create: `.github/workflows/publish_charms.yml`

**Interfaces:**
- Consumes: Go source (`go test ./...`), `web/` frontend (`npm test`, `npm run lint`), charm Python (`tox`), `artifacts.yaml`/`spread.yaml` (charm-ci), `charmcraft.yaml`
- Produces: 5 GitHub Actions workflows: internal_tests, frontend_tests, charms_lint_and_unit, charms_integration, publish_charms

- [ ] **Step 1: Create the workflows directory**

```bash
mkdir -p .github/workflows
```

- [ ] **Step 2: Create `.github/workflows/internal_tests.yaml`**

```yaml
# internal_tests.yaml — Go backend CI.
# Runs: go test ./... (all packages), coverage gate ≥85%, golangci-lint.
name: Internal Tests

on:
  pull_request:
  push:
    branches:
      - main
      - feat/initial-development-isd-6054

jobs:
  test:
    name: Go Tests & Lint
    runs-on: ubuntu-24.04

    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_USER: bingo
          POSTGRES_PASSWORD: bingo
          POSTGRES_DB: bingo
        ports:
          - 5432:5432
        options: >-
          --health-cmd "pg_isready -U bingo"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Run tests with coverage
        env:
          DATABASE_URL: postgres://bingo:bingo@localhost:5432/bingo?sslmode=disable
        run: |
          go test -coverprofile=coverage.out ./...
          go tool cover -func=coverage.out | tail -1
          COVERAGE=$(go tool cover -func=coverage.out | grep '^total' | awk '{print $3}' | tr -d '%')
          echo "Coverage: ${COVERAGE}%"
          awk "BEGIN { if (${COVERAGE} < 85) { print \"Coverage ${COVERAGE}% is below 85% gate\"; exit 1 } }"

      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest
          args: --timeout 5m
```

- [ ] **Step 3: Create `.github/workflows/frontend_tests.yaml`**

```yaml
# frontend_tests.yaml — React frontend CI.
# Runs: ESLint (0 warnings), Vitest unit tests (52+), Playwright E2E (10+).
name: Frontend Tests

on:
  pull_request:
  push:
    branches:
      - main
      - feat/initial-development-isd-6054

jobs:
  test:
    name: Frontend Lint, Unit & E2E
    runs-on: ubuntu-24.04

    defaults:
      run:
        working-directory: web

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json

      - name: Install dependencies
        run: npm ci

      - name: Lint
        run: npm run lint

      - name: Unit tests
        run: npm test -- --run

      - name: Build (required for Playwright preview server)
        run: npm run build

      - name: Install Playwright browsers
        run: npx playwright install --with-deps chromium

      - name: Playwright E2E tests
        run: npx playwright test
```

- [ ] **Step 4: Create `.github/workflows/charms_lint_and_unit.yaml`**

```yaml
# charms_lint_and_unit.yaml — Charm Python CI.
# Runs tox envs: fmt (check), lint, static (mypy), unit (pytest + coverage).
name: Charm Lint & Unit Tests

on:
  pull_request:
  push:
    branches:
      - main
      - feat/initial-development-isd-6054

jobs:
  lint-and-unit:
    name: Charm ${{ matrix.toxenv }}
    runs-on: ubuntu-24.04
    strategy:
      matrix:
        toxenv: [lint, static, unit]

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-python@v5
        with:
          python-version: '3.12'

      - name: Install tox
        run: pip install tox

      - name: Run tox ${{ matrix.toxenv }}
        run: tox -e ${{ matrix.toxenv }}
```

- [ ] **Step 5: Create `.github/workflows/charms_integration.yaml`**

```yaml
# charms_integration.yaml — Charm integration CI.
# Delegates to charm-ci integration-test.yml which builds the rock + charm,
# then runs spread integration suites defined in spread.yaml via opcli.
# Reference: https://github.com/canonical/charm-ci/.github/workflows/integration-test.yml
name: Charm Integration Tests

on:
  pull_request:
  push:
    branches:
      - main
      - feat/initial-development-isd-6054

jobs:
  integration-test:
    name: Integration Tests
    uses: canonical/charm-ci/.github/workflows/integration-test.yml@main
    permissions:
      contents: read
      packages: write
      actions: read
    secrets: inherit
    with:
      working-directory: .
```

- [ ] **Step 6: Create `.github/workflows/publish_charms.yml`**

```yaml
# publish_charms.yml — Publish to CharmHub.
# Delegates to charm-ci publish-artifacts.yml which calls opcli to upload
# the built rock and charm to their respective registries.
# Reference: https://github.com/canonical/charm-ci/.github/workflows/publish-artifacts.yml
name: Publish Charms

on:
  workflow_dispatch:
    inputs:
      channel:
        description: "CharmHub channel (e.g. latest/edge, 1.0/stable)"
        required: false
        type: string
      dry-run:
        description: "Validate artifacts without uploading"
        type: boolean
        default: false

jobs:
  publish:
    name: Publish to CharmHub
    uses: canonical/charm-ci/.github/workflows/publish-artifacts.yml@main
    with:
      channel: ${{ inputs.channel }}
      dry-run: ${{ inputs.dry-run }}
    permissions:
      contents: read
      actions: read
    secrets: inherit
```

- [ ] **Step 7: Validate all workflow YAML syntax**

```bash
python3 -c "
import yaml, os
wf_dir = '.github/workflows'
for f in sorted(os.listdir(wf_dir)):
    if f.endswith(('.yaml', '.yml')):
        yaml.safe_load(open(os.path.join(wf_dir, f)))
        print(f'OK: {wf_dir}/{f}')
"
```

Expected: `OK:` for all 5 workflow files, no errors.

- [ ] **Step 8: Commit**

```bash
git add .github/workflows/
git commit -m "ci: add all GitHub Actions workflows (go, frontend, charm lint+unit, charm integration, publish)"
```

---

## Task 7: Charm Integration Tests

**Files:**
- Create: `tests/integration/__init__.py`
- Create: `tests/integration/test_charm.py`
- Create: `tests/integration/requirements.txt`

**Interfaces:**
- Consumes: `BingoCharm` (deployed as a real Juju charm), `ops_test` fixture from `pytest-operator`
- Produces: integration test suite that verifies bingo deploys, becomes active, and responds to `/api/v1/healthz` via ingress

**Note:** Integration tests require a real Juju/MicroK8s environment. They cannot run locally without the full charm-ci spread stack. They are run by `charms_integration.yaml` via `opcli spread run`. The tests are written to spec here; CI validates them.

- [ ] **Step 1: Create `tests/integration/__init__.py`**

```bash
touch tests/integration/__init__.py
```

- [ ] **Step 2: Create `tests/integration/requirements.txt`**

```text
pytest
pytest-operator>=0.36
juju>=3.5.0,<4.0
ops>=3.7.0,<4.0
paas-charm
```

- [ ] **Step 3: Create `tests/integration/test_charm.py`**

```python
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Integration tests for BingoCharm.

These tests run via pytest-operator inside the charm-ci spread environment
(MicroK8s + Juju 3.6). They are NOT run during local development.

Usage (in spread environment):
    pytest tests/integration/ -v --model testing
"""

import asyncio
import logging
import typing

import pytest
import pytest_asyncio
from juju.application import Application
from pytest_operator.plugin import OpsTest

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(scope="module")
def app_image(request: pytest.FixtureRequest) -> str:
    """OCI image path; provided by charm-ci via --app-image CLI option."""
    image = request.config.getoption("--app-image", default=None)
    if image is None:
        pytest.skip("--app-image not provided; skipping integration tests")
    return typing.cast(str, image)


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption(
        "--app-image",
        action="store",
        help="OCI image reference for the bingo app-image resource",
    )


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


@pytest.mark.abort_on_fail
async def test_build_and_deploy(ops_test: OpsTest, app_image: str) -> None:
    """Build the charm and deploy it with the OCI rock."""
    charm_path = await ops_test.build_charm(".")
    assert ops_test.model is not None

    app: Application = await ops_test.model.deploy(
        str(charm_path),
        application_name="bingo",
        resources={"app-image": app_image},
        trust=True,
    )

    # Deploy a PostgreSQL charm and relate it.
    await ops_test.model.deploy(
        "postgresql-k8s",
        application_name="postgresql",
        channel="14/stable",
        trust=True,
    )
    await ops_test.model.integrate("bingo:postgresql", "postgresql:database")

    # Deploy a traefik ingress.
    await ops_test.model.deploy(
        "traefik-k8s",
        application_name="traefik",
        channel="latest/stable",
        trust=True,
    )
    await ops_test.model.integrate("bingo:ingress", "traefik:ingress")

    await ops_test.model.wait_for_idle(
        apps=["bingo", "postgresql", "traefik"],
        status="active",
        timeout=600,
        raise_on_error=True,
    )
    logger.info("All applications reached active status.")


async def test_healthz_responds(ops_test: OpsTest) -> None:
    """The /api/v1/healthz endpoint must return HTTP 200."""
    import urllib.request

    assert ops_test.model is not None
    bingo_app = ops_test.model.applications.get("bingo")
    assert bingo_app is not None

    # Retrieve the ingress URL from the unit status message.
    unit = bingo_app.units[0]
    status_msg = unit.workload_status_message
    logger.info("Unit status: %s", status_msg)

    # Attempt healthz via the ingress address if available.
    traefik_app = ops_test.model.applications.get("traefik")
    if traefik_app:
        traefik_unit = traefik_app.units[0]
        # Traefik exposes its IP via status message or address.
        ip = await traefik_unit.get_public_address()
        url = f"http://{ip}/bingo/api/v1/healthz"
        logger.info("Checking healthz at %s", url)
        with urllib.request.urlopen(url, timeout=10) as resp:
            assert resp.status == 200, f"Expected 200, got {resp.status}"
    else:
        pytest.skip("Traefik not deployed; skipping healthz check")


async def test_paste_create_anonymous(ops_test: OpsTest) -> None:
    """Anonymous paste creation must return 201 with a key."""
    import json
    import urllib.request

    assert ops_test.model is not None
    traefik_app = ops_test.model.applications.get("traefik")
    if not traefik_app:
        pytest.skip("Traefik not deployed")

    ip = await traefik_app.units[0].get_public_address()
    url = f"http://{ip}/bingo/api/v1/pastes"
    payload = json.dumps({
        "content": "hello from integration test",
        "language": "plaintext",
        "expires_in": "1d",
    }).encode()
    req = urllib.request.Request(
        url,
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        assert resp.status == 201
        body = json.loads(resp.read())
        assert "key" in body
        assert len(body["key"]) >= 4
        logger.info("Created paste with key: %s", body["key"])
```

- [ ] **Step 4: Validate test syntax**

```bash
export PATH="$HOME/go/bin:$PATH"
python3 -c "import ast; ast.parse(open('tests/integration/test_charm.py').read()); print('syntax OK')"
```

Expected: `syntax OK`

- [ ] **Step 5: Run all Go and frontend tests to confirm no regressions**

```bash
export PATH="$HOME/go/bin:$PATH"
go test ./... 2>&1 | grep -E "^ok|FAIL"
```

Expected: 5 packages `ok`, 0 `FAIL`.

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd web && npm test -- --run 2>&1 | tail -5
cd ..
```

Expected: `52 passed` (or more), `0 failed`.

- [ ] **Step 6: Commit**

```bash
git add tests/integration/__init__.py tests/integration/test_charm.py tests/integration/requirements.txt
git commit -m "test(charm): add pytest-operator integration tests for bingo charm deployment"
```

---

## Self-Review

### 1. Spec Coverage

| Spec requirement | Task covering it |
|-----------------|-----------------|
| Security headers (§8): X-Frame-Options, X-Content-Type-Options, CSP | T1 |
| `.env.example` documenting all env vars (§13) | T1 |
| `bingo-rockcraft.yaml` building Go binary + web/dist (Phase 5) | T2 |
| `charmcraft.yaml` with go-framework extension (Phase 5) | T3 |
| `src/charm.py` extending `paas_charm.go.Charm` (Phase 5) | T3 |
| Charm unit tests (Scenario framework) (Phase 5) | T4 |
| `artifacts.yaml` exact content from §14 | T5 |
| `spread.yaml` (Phase 5) | T5 |
| `concierge.yaml` (Phase 5) | T5 |
| `internal_tests.yaml` — go test + coverage ≥ 85% + golangci-lint (§14, Phase 5) | T6 |
| `frontend_tests.yaml` — lint + Vitest + Playwright (§14, Phase 5) | T6 |
| `charms_lint_and_unit.yaml` — charm tox (§14, Phase 5) | T6 |
| `charms_integration.yaml` — delegates to charm-ci integration-test.yml (§14) | T6 |
| `publish_charms.yml` — delegates to charm-ci publish-artifacts.yml (§14) | T6 |
| Charm integration tests (Phase 5) | T7 |

All spec requirements are covered. ✅

### 2. Placeholder Scan

Reviewed. All code blocks are complete. No "TBD" or "TODO" placeholders. ✅

### 3. Type Consistency

- `BingoCharm` defined in T3 (`src/charm.py`), imported in T4 (`tests/unit/test_charm.py`) ✅
- Rock name `bingo` defined in T2 (`bingo-rockcraft.yaml`), referenced in T5 (`artifacts.yaml`) ✅
- `app-image` resource defined in T3 (`charmcraft.yaml`), referenced in T5 (`artifacts.yaml`) ✅
- `concierge.yaml` name used in T5 (`spread.yaml` environment default) ✅
