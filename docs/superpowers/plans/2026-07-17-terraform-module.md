# bingo Reference Terraform Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a reusable Juju Terraform Provider module for the bingo charm: a
base module (`terraform/`) that deploys the `bingo` application only, and a
product module (`terraform/product/`) that owns the model, bundles the
postgresql/oauth/ingress dependency charms, and wires all relations (including
offer-URL-only integrations for tracing/logging/metrics/grafana-dashboard).

**Architecture:** Two Terraform modules following the Canonical IS
charm-Terraform-module convention (mirrors `canonical/mattermost-k8s-operator`
PR #70). The base module wraps a single `juju_application` resource and exposes
`requires`/`provides` endpoint-name maps. The product module creates the
`juju_model`, calls the base module, conditionally deploys `postgresql-k8s`,
`oauth-external-idp-integrator`, and `traefik-k8s` (each behind a `deploy_*`
bool), and creates `juju_integration` resources for both local bundled apps and
external cross-model offers.

**Tech Stack:** Terraform >= 1.6.0, the `juju/juju` Terraform provider,
`terraform test` (`.tftest.hcl`), `tflint`.

## Global Constraints

- Design spec: `docs/superpowers/specs/2026-07-17-terraform-module-design.md` —
  follow it exactly; this plan implements it task-by-task.
- No live Juju controller or k8s cloud is available in the development
  environment. Validation here is structural only: `terraform fmt -check`,
  `terraform init -backend=false`, `terraform validate`, and `tflint`. Do NOT
  attempt `terraform apply` or a real `terraform test` run (`command = plan`
  runs require a real provider connection) — real validation happens in CI.
- `config` variables in the base module are a generic `map(string)` passthrough
  — do not add one Terraform variable per charm config key.
- Base module never creates a `juju_model` or `juju_integration` — only the
  product module does.
- Bundled dependency charms: `postgresql-k8s` (postgresql),
  `oauth-external-idp-integrator` (oauth), `traefik-k8s` (ingress). Tracing,
  logging, metrics-endpoint, and grafana-dashboard are offer-URL-only — no
  charm is bundled for them.
- Relation endpoint names (from `charm/charmcraft.yaml` and the go-framework
  extension, confirmed against `charmcraft/extensions/app.py`): requires =
  `postgresql`, `oauth`, `tracing`, `ingress`, `logging`; provides =
  `metrics-endpoint`, `grafana-dashboard`. Note `logging` (interface
  `loki_push_api`) and `ingress` are both `requires`, not `provides` — bingo
  requires a Loki/ingress endpoint to push logs to / receive traffic through.
- `juju_integration` `application` blocks use either `{ name, endpoint }` (local
  app) or `{ offer_url }` alone (no `endpoint` field when using `offer_url`).

---

## Task 1: Install Terraform tooling and scaffold the base module core resource

**Files:**
- Create: `terraform/versions.tf`
- Create: `terraform/variables.tf`
- Create: `terraform/main.tf`
- Create: `terraform/outputs.tf`

**Interfaces:**
- Produces: `juju_application.bingo` resource; variables `app_name` (string,
  default `"bingo"`), `model_uuid` (string, required), `base` (string, default
  `"ubuntu@24.04"`), `channel` (string, default `"latest/edge"`), `revision`
  (number, default `null`), `config` (map(string), default `{}`), `constraints`
  (string, default `""`), `units` (number, default `1`); outputs `app_name`,
  `requires` (map with keys `postgresql`, `oauth`, `tracing`, `ingress`,
  `logging`), `provides` (map with keys `metrics_endpoint`,
  `grafana_dashboard`).

- [ ] **Step 1: Install `terraform` and `tflint` CLIs (if not already present)**

```bash
which terraform || {
  curl -fsSL -o /tmp/terraform.zip https://releases.hashicorp.com/terraform/1.9.8/terraform_1.9.8_linux_amd64.zip
  unzip -o /tmp/terraform.zip -d /tmp/tfbin
  sudo mv /tmp/tfbin/terraform /usr/local/bin/terraform
}
terraform version

which tflint || curl -s https://raw.githubusercontent.com/terraform-linters/tflint/master/install_linux.sh | bash
tflint --version
```

Expected: both commands print version info without error.

- [ ] **Step 2: Create `terraform/versions.tf`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    juju = {
      source  = "juju/juju"
      version = ">= 0.19.0"
    }
  }
}
```

- [ ] **Step 3: Create `terraform/variables.tf`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

variable "app_name" {
  description = "Name of the application in the Juju model."
  type        = string
  default     = "bingo"
}

variable "model_uuid" {
  description = "UUID of the Juju model where the application will be deployed. The model must already exist; this module does not create one."
  type        = string
}

variable "base" {
  description = "The operating system base on which to deploy the charm."
  type        = string
  default     = "ubuntu@24.04"
}

variable "channel" {
  description = "The Charmhub channel to use when deploying the bingo charm."
  type        = string
  default     = "latest/edge"
}

variable "revision" {
  description = "Revision number of the bingo charm. Leave null to use the latest revision in the given channel."
  type        = number
  default     = null
}

variable "config" {
  description = <<-EOT
    Application config, passed through directly to the bingo charm. Keys match
    the options documented in charm/charmcraft.yaml, e.g. "base-url",
    "max-paste-size-bytes", "log-level", "web-dir", "oauth-redirect-path",
    "oauth-scopes", "oauth-user-name-attribute".
  EOT
  type        = map(string)
  default     = {}
}

variable "constraints" {
  description = "Juju constraints to apply for this application."
  type        = string
  default     = ""
}

variable "units" {
  description = "Number of units to deploy."
  type        = number
  default     = 1
}
```

- [ ] **Step 4: Create `terraform/main.tf`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

resource "juju_application" "bingo" {
  name       = var.app_name
  model_uuid = var.model_uuid

  charm {
    name     = "bingo"
    channel  = var.channel
    revision = var.revision
    base     = var.base
  }

  config      = var.config
  constraints = var.constraints
  units       = var.units
}
```

- [ ] **Step 5: Create `terraform/outputs.tf`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

output "app_name" {
  description = "Name of the deployed bingo application."
  value       = juju_application.bingo.name
}

output "requires" {
  description = "Map of bingo's `requires` relation names to their endpoint names."
  value = {
    postgresql = "postgresql"
    oauth      = "oauth"
    tracing    = "tracing"
    ingress    = "ingress"
    logging    = "logging"
  }
}

output "provides" {
  description = "Map of bingo's `provides` relation names to their endpoint names."
  value = {
    metrics_endpoint  = "metrics-endpoint"
    grafana_dashboard = "grafana-dashboard"
  }
}
```

- [ ] **Step 6: Validate the module structurally**

```bash
cd terraform
terraform fmt -check
terraform init -backend=false
terraform validate
```

Expected: `fmt -check` prints nothing (already formatted); `init` succeeds
downloading the `juju` provider; `validate` prints `Success! The configuration
is valid.`

- [ ] **Step 7: Commit**

```bash
git add terraform/versions.tf terraform/variables.tf terraform/main.tf terraform/outputs.tf
git commit -m "feat: scaffold bingo base terraform module"
```

---

## Task 2: Add base module lint config and `terraform test` suite

**Files:**
- Create: `terraform/.tflint.hcl`
- Create: `terraform/tests/setup/main.tf`
- Create: `terraform/tests/main.tftest.hcl`

**Interfaces:**
- Consumes: `terraform/variables.tf`, `terraform/outputs.tf` from Task 1
  (`app_name`, `model_uuid`, `channel`, `revision`, `requires`).
- Produces: `tests/setup` module outputs `model_uuid` (string), `model_name`
  (string) — for future CI use only; not runnable here without a real cloud.

- [ ] **Step 1: Create `terraform/.tflint.hcl`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

rule "terraform_required_version" {
  enabled = false
}
```

- [ ] **Step 2: Create `terraform/tests/setup/main.tf`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

terraform {
  required_providers {
    juju = {
      source  = "juju/juju"
      version = ">= 0.19.0"
    }
  }
}

provider "juju" {}

resource "juju_model" "test_model" {
  name       = "tf-bingo-${formatdate("YYYYMMDDhhmmss", timestamp())}"
  credential = "tfk8s"

  cloud {
    name = "tfk8s"
  }
}

output "model_uuid" {
  value = juju_model.test_model.uuid
}

output "model_name" {
  value = juju_model.test_model.name
}
```

- [ ] **Step 3: Create `terraform/tests/main.tftest.hcl`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

provider "juju" {}

run "setup_tests" {
  module {
    source = "./tests/setup"
  }
}

run "basic_deploy" {
  command = plan

  variables {
    app_name   = "bingo"
    model_uuid = run.setup_tests.model_uuid
    channel    = "latest/edge"
    revision   = null
  }

  assert {
    condition     = output.app_name == "bingo"
    error_message = "bingo app_name did not match expected"
  }

  assert {
    condition     = output.requires.postgresql == "postgresql"
    error_message = "postgresql endpoint name did not match expected"
  }
}
```

- [ ] **Step 4: Validate structurally (this environment has no real cloud, so
  only run `validate`/`fmt`/`tflint`, NOT `terraform test`)**

```bash
cd terraform
terraform fmt -check -recursive
terraform validate
cd tests/setup && terraform init -backend=false && terraform validate && cd ../..
tflint --chdir=. --recursive
```

Expected: `fmt -check` prints nothing; both `validate` calls print `Success!
The configuration is valid.`; `tflint` reports no errors (it will only see the
disabled `terraform_required_version` rule from `.tflint.hcl`).

- [ ] **Step 5: Commit**

```bash
git add terraform/.tflint.hcl terraform/tests
git commit -m "test: add terraform test suite and tflint config for base module"
```

---

## Task 3: Write the base module README

**Files:**
- Create: `terraform/README.md`

**Interfaces:**
- Consumes: variable and output names from Tasks 1–2 (must document them
  exactly as named: `app_name`, `model_uuid`, `base`, `channel`, `revision`,
  `config`, `constraints`, `units`; outputs `app_name`, `requires`, `provides`).

- [ ] **Step 1: Create `terraform/README.md`**

```markdown
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
- Observability integrations (Loki logging, Prometheus metrics-endpoint,
  Grafana dashboards) are all optional and provided by bingo, not required.

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
```

- [ ] **Step 2: Commit**

```bash
git add terraform/README.md
git commit -m "docs: add bingo base terraform module README"
```

---

## Task 4: Scaffold the product module core resources

**Files:**
- Create: `terraform/product/versions.tf`
- Create: `terraform/product/variables.tf`
- Create: `terraform/product/main.tf`
- Create: `terraform/product/outputs.tf`

**Interfaces:**
- Consumes: base module at `../` — `module.bingo.app_name`,
  `module.bingo.requires.{postgresql,oauth,tracing,ingress,logging}`,
  `module.bingo.provides.{metrics_endpoint,grafana_dashboard}`.
- Produces: outputs `model_name` (string), `bingo` (object with `app_name` and
  `requires`), `postgresql_app_name` (string or null),
  `oauth_app_name` (string or null), `ingress_app_name` (string or null).

- [ ] **Step 1: Create `terraform/product/versions.tf`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    juju = {
      source  = "juju/juju"
      version = ">= 0.19.0"
    }
  }
}
```

- [ ] **Step 2: Create `terraform/product/variables.tf`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

variable "model_name" {
  description = "Name of the Juju model to create for this deployment."
  type        = string
  default     = "bingo"
}

variable "cloud_name" {
  description = "Name of the Juju cloud to deploy the model onto."
  type        = string
}

variable "credential_name" {
  description = "Name of the Juju credential to use for the model. Leave null to use the cloud's default credential."
  type        = string
  default     = null
}

variable "bingo" {
  description = "bingo charm configuration."
  type = object({
    app_name = optional(string, "bingo")
    channel  = optional(string, "latest/edge")
    revision = optional(number, null)
    base     = optional(string, "ubuntu@24.04")
    config   = optional(map(string), {})
    units    = optional(number, 1)
  })
  default = {}
}

variable "deploy_postgresql" {
  description = "Whether to deploy the bundled postgresql-k8s charm. Set to false to integrate an existing PostgreSQL application or offer instead (not managed by this module in that case)."
  type        = bool
  default     = true
}

variable "postgresql" {
  description = "PostgreSQL K8s charm configuration (used when deploy_postgresql is true)."
  type = object({
    channel  = optional(string, "14/stable")
    revision = optional(number, null)
    config   = optional(map(string), {})
    units    = optional(number, 1)
  })
  default = {}
}

variable "deploy_oauth" {
  description = "Whether to deploy the bundled oauth-external-idp-integrator charm. Set to false to integrate an existing oauth application or offer instead."
  type        = bool
  default     = true
}

variable "oauth" {
  description = "oauth-external-idp-integrator charm configuration. Its config must describe a real external identity provider (issuer URL, client ID/secret, etc.) when deploy_oauth is true."
  type = object({
    channel  = optional(string, "latest/edge")
    revision = optional(number, null)
    config   = optional(map(string), {})
  })
  default = {}
}

variable "deploy_ingress" {
  description = "Whether to deploy the bundled traefik-k8s charm. Set to false to integrate an existing ingress application or offer instead."
  type        = bool
  default     = true
}

variable "traefik" {
  description = "traefik-k8s charm configuration (used when deploy_ingress is true)."
  type = object({
    channel  = optional(string, "latest/stable")
    revision = optional(number, null)
    config   = optional(map(string), {})
  })
  default = {}
}

variable "tracing_offer_url" {
  description = "Juju offer URL for an existing tracing provider (e.g. Charmed Tempo). When set, bingo's tracing endpoint is integrated to this offer. Leave null to skip tracing entirely."
  type        = string
  default     = null
}

variable "logging_offer_url" {
  description = "Juju offer URL for an existing Loki logging provider. When set, bingo's logging endpoint is integrated to this offer. Leave null to skip."
  type        = string
  default     = null
}

variable "metrics_offer_url" {
  description = "Juju offer URL for an existing Prometheus metrics scraper. When set, bingo's metrics-endpoint is integrated to this offer. Leave null to skip."
  type        = string
  default     = null
}

variable "grafana_dashboard_offer_url" {
  description = "Juju offer URL for an existing Grafana dashboard provider. When set, bingo's grafana-dashboard endpoint is integrated to this offer. Leave null to skip."
  type        = string
  default     = null
}
```

- [ ] **Step 3: Create `terraform/product/main.tf`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

resource "juju_model" "this" {
  name = var.model_name

  cloud {
    name = var.cloud_name
  }

  credential = var.credential_name
}

module "bingo" {
  source     = "../"
  model_uuid = juju_model.this.uuid
  app_name   = var.bingo.app_name
  channel    = var.bingo.channel
  revision   = var.bingo.revision
  base       = var.bingo.base
  config     = var.bingo.config
  units      = var.bingo.units
}

# --- Bundled dependency charms ---

resource "juju_application" "postgresql" {
  count      = var.deploy_postgresql ? 1 : 0
  name       = "postgresql-k8s"
  model_uuid = juju_model.this.uuid

  charm {
    name     = "postgresql-k8s"
    channel  = var.postgresql.channel
    revision = var.postgresql.revision
  }

  config = var.postgresql.config
  trust  = true
  units  = var.postgresql.units
}

resource "juju_application" "oauth" {
  count      = var.deploy_oauth ? 1 : 0
  name       = "oauth-external-idp-integrator"
  model_uuid = juju_model.this.uuid

  charm {
    name     = "oauth-external-idp-integrator"
    channel  = var.oauth.channel
    revision = var.oauth.revision
  }

  config = var.oauth.config
  units  = 1
}

resource "juju_application" "traefik" {
  count      = var.deploy_ingress ? 1 : 0
  name       = "traefik-k8s"
  model_uuid = juju_model.this.uuid

  charm {
    name     = "traefik-k8s"
    channel  = var.traefik.channel
    revision = var.traefik.revision
  }

  config = var.traefik.config
  trust  = true
  units  = 1
}

# --- Integrations: bundled dependencies ---

resource "juju_integration" "bingo_postgresql" {
  count      = var.deploy_postgresql ? 1 : 0
  model_uuid = juju_model.this.uuid

  application {
    name     = module.bingo.app_name
    endpoint = module.bingo.requires.postgresql
  }

  application {
    name     = juju_application.postgresql[0].name
    endpoint = "database"
  }
}

resource "juju_integration" "bingo_oauth" {
  count      = var.deploy_oauth ? 1 : 0
  model_uuid = juju_model.this.uuid

  application {
    name     = module.bingo.app_name
    endpoint = module.bingo.requires.oauth
  }

  application {
    name     = juju_application.oauth[0].name
    endpoint = "oauth"
  }
}

resource "juju_integration" "bingo_ingress" {
  count      = var.deploy_ingress ? 1 : 0
  model_uuid = juju_model.this.uuid

  application {
    name     = module.bingo.app_name
    endpoint = module.bingo.requires.ingress
  }

  application {
    name     = juju_application.traefik[0].name
    endpoint = "ingress"
  }
}

# --- Integrations: external offers (no bundled charm) ---

resource "juju_integration" "bingo_tracing" {
  count      = var.tracing_offer_url != null ? 1 : 0
  model_uuid = juju_model.this.uuid

  application {
    name     = module.bingo.app_name
    endpoint = module.bingo.requires.tracing
  }

  application {
    offer_url = var.tracing_offer_url
  }
}

resource "juju_integration" "bingo_logging" {
  count      = var.logging_offer_url != null ? 1 : 0
  model_uuid = juju_model.this.uuid

  application {
    name     = module.bingo.app_name
    endpoint = module.bingo.requires.logging
  }

  application {
    offer_url = var.logging_offer_url
  }
}

resource "juju_integration" "bingo_metrics" {
  count      = var.metrics_offer_url != null ? 1 : 0
  model_uuid = juju_model.this.uuid

  application {
    name     = module.bingo.app_name
    endpoint = module.bingo.provides.metrics_endpoint
  }

  application {
    offer_url = var.metrics_offer_url
  }
}

resource "juju_integration" "bingo_grafana_dashboard" {
  count      = var.grafana_dashboard_offer_url != null ? 1 : 0
  model_uuid = juju_model.this.uuid

  application {
    name     = module.bingo.app_name
    endpoint = module.bingo.provides.grafana_dashboard
  }

  application {
    offer_url = var.grafana_dashboard_offer_url
  }
}
```

- [ ] **Step 4: Create `terraform/product/outputs.tf`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

output "model_name" {
  description = "Name of the Juju model created for this deployment."
  value       = juju_model.this.name
}

output "bingo" {
  description = "bingo application name and relation endpoint names."
  value = {
    app_name = module.bingo.app_name
    requires = module.bingo.requires
    provides = module.bingo.provides
  }
}

output "postgresql_app_name" {
  description = "Name of the deployed PostgreSQL application, if bundled (deploy_postgresql = true)."
  value       = one(juju_application.postgresql[*].name)
}

output "oauth_app_name" {
  description = "Name of the deployed oauth-external-idp-integrator application, if bundled (deploy_oauth = true)."
  value       = one(juju_application.oauth[*].name)
}

output "ingress_app_name" {
  description = "Name of the deployed traefik-k8s application, if bundled (deploy_ingress = true)."
  value       = one(juju_application.traefik[*].name)
}
```

- [ ] **Step 5: Validate the product module structurally**

```bash
cd terraform/product
terraform fmt -check
terraform init -backend=false
terraform validate
```

Expected: `fmt -check` prints nothing; `init` succeeds (resolves both the
`juju` provider and the local `../` module source); `validate` prints
`Success! The configuration is valid.`

- [ ] **Step 6: Commit**

```bash
git add terraform/product/versions.tf terraform/product/variables.tf terraform/product/main.tf terraform/product/outputs.tf
git commit -m "feat: scaffold bingo product terraform module"
```

---

## Task 5: Add product module lint config and `terraform test` suite

**Files:**
- Create: `terraform/product/.tflint.hcl`
- Create: `terraform/product/tests/setup/main.tf`
- Create: `terraform/product/tests/main.tftest.hcl`

**Interfaces:**
- Consumes: `terraform/product/variables.tf` and `outputs.tf` from Task 4
  (`model_name`, `cloud_name`, `deploy_postgresql`, `deploy_oauth`,
  `deploy_ingress`, `bingo`; outputs `model_name`, `bingo`).

- [ ] **Step 1: Create `terraform/product/.tflint.hcl`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

rule "terraform_required_version" {
  enabled = false
}
```

- [ ] **Step 2: Create `terraform/product/tests/setup/main.tf`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

terraform {
  required_providers {
    juju = {
      source  = "juju/juju"
      version = ">= 0.19.0"
    }
  }
}

provider "juju" {}

output "cloud_name" {
  value = "tfk8s"
}
```

- [ ] **Step 3: Create `terraform/product/tests/main.tftest.hcl`**

```hcl
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

provider "juju" {}

run "setup_tests" {
  module {
    source = "./tests/setup"
  }
}

run "basic_deploy" {
  command = plan

  variables {
    model_name        = "tf-bingo-product-test"
    cloud_name        = run.setup_tests.cloud_name
    deploy_postgresql = true
    deploy_oauth      = false
    deploy_ingress    = false
  }

  assert {
    condition     = output.model_name == "tf-bingo-product-test"
    error_message = "model_name did not match expected"
  }

  assert {
    condition     = output.bingo.app_name == "bingo"
    error_message = "bingo app_name did not match expected"
  }
}
```

- [ ] **Step 4: Validate structurally**

```bash
cd terraform/product
terraform fmt -check -recursive
terraform validate
cd tests/setup && terraform init -backend=false && terraform validate && cd ../..
tflint --chdir=. --recursive
```

Expected: `fmt -check` prints nothing; both `validate` calls print `Success!
The configuration is valid.`; `tflint` reports no errors.

- [ ] **Step 5: Commit**

```bash
git add terraform/product/.tflint.hcl terraform/product/tests
git commit -m "test: add terraform test suite and tflint config for product module"
```

---

## Task 6: Write the product module README

**Files:**
- Create: `terraform/product/README.md`

**Interfaces:**
- Consumes: variable and output names from Task 4 (`model_name`, `cloud_name`,
  `credential_name`, `bingo`, `deploy_postgresql`, `postgresql`,
  `deploy_oauth`, `oauth`, `deploy_ingress`, `traefik`, `tracing_offer_url`,
  `logging_offer_url`, `metrics_offer_url`, `grafana_dashboard_offer_url`;
  outputs `model_name`, `bingo`, `postgresql_app_name`, `oauth_app_name`,
  `ingress_app_name`).

- [ ] **Step 1: Create `terraform/product/README.md`**

```markdown
# bingo product Terraform module

This folder contains a self-contained [Terraform][Terraform] module that
deploys a working `bingo` model, including bundled dependency charms and all
relations. It is the "reusable building block" referenced by the bingo
Terraform module ticket: apply it and get a running bingo deployment without
reading the charm source.

Unlike the [base module](../README.md), this module:

- Creates the Juju **model** itself (`juju_model.this`).
- Bundles deployable backend charms for `postgresql`, `oauth`, and `ingress`
  (each optional, controlled by a `deploy_*` flag).
- Wires `tracing`, `logging`, `metrics-endpoint`, and `grafana-dashboard` only
  via externally-supplied Juju **offer URLs** — no charm is bundled for these,
  since they require pointing at existing observability infrastructure (e.g.
  a Canonical Observability Stack deployment).

## Module structure

- **main.tf** - Creates the model, instantiates the [base module](../), and
  conditionally deploys `postgresql-k8s`, `oauth-external-idp-integrator`, and
  `traefik-k8s`, plus all `juju_integration` resources.
- **variables.tf** - Model settings, bingo charm config, per-dependency
  `deploy_*` flags and config objects, and offer URLs for tracing/logging/
  metrics/grafana-dashboard.
- **outputs.tf** - Model name, bingo app name/endpoints, and the names of any
  bundled dependency applications.
- **versions.tf** - Pins the required Terraform and `juju` provider versions.
- **tests/** - `terraform test` suite; requires a real Juju controller and
  Kubernetes cloud (intended for CI).

## Inputs

| Name | Type | Default | Description |
|---|---|---|---|
| `model_name` | `string` | `"bingo"` | Name of the Juju model to create. |
| `cloud_name` | `string` | *(required)* | Name of the Juju cloud to deploy the model onto. |
| `credential_name` | `string` | `null` | Juju credential to use; `null` uses the cloud's default. |
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
| `model_name` | Name of the created Juju model. |
| `bingo` | `{ app_name, requires, provides }` — bingo's application name and relation endpoint name maps. |
| `postgresql_app_name` | Name of the bundled PostgreSQL application, or `null` if `deploy_postgresql = false`. |
| `oauth_app_name` | Name of the bundled oauth-external-idp-integrator application, or `null` if `deploy_oauth = false`. |
| `ingress_app_name` | Name of the bundled traefik-k8s application, or `null` if `deploy_ingress = false`. |

## Required external inputs

Before you can `terraform apply` a fully working deployment you need:

1. A Juju cloud (`cloud_name`) and, if not using the default, a credential
   (`credential_name`) already added to your Juju controller.
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
module "bingo" {
  source = "git::https://github.com/canonical/bingo//terraform/product"

  cloud_name = "my-k8s-cloud"
  model_name = "bingo-production"

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
```

- [ ] **Step 2: Commit**

```bash
git add terraform/product/README.md
git commit -m "docs: add bingo product terraform module README"
```

---

## Task 7: Add CI workflow and link from the root README

**Files:**
- Create: `.github/workflows/test_terraform_modules.yaml`
- Modify: `README.md` (add a "Terraform module" section near the end, before
  "Project and community")

**Interfaces:**
- None (this task wires CI and documentation only; no Terraform code changes).

- [ ] **Step 1: Create `.github/workflows/test_terraform_modules.yaml`**

```yaml
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

name: Terraform modules tests

on:
  workflow_dispatch:
  pull_request:
    paths:
      - 'terraform/**'

jobs:
  terraform-tests:
    uses: canonical/operator-workflows/.github/workflows/terraform_modules_test.yaml@main
    secrets: inherit
    with:
      k8s-controller: true
      terraform-directories: '["terraform", "terraform/product"]'
```

- [ ] **Step 2: Add a "Terraform module" section to `README.md`**

Find this section in `README.md`:

```markdown
## Project and community
```

Replace it with (adding the new section directly above it):

```markdown
## Terraform module

A reusable [Terraform][terraform] module for deploying bingo via the
[Juju Terraform provider][juju-terraform-provider] is available in
[`terraform/`](terraform/README.md) (application only) and
[`terraform/product/`](terraform/product/README.md) (model + application +
bundled dependencies + relations — a full working deployment).

[terraform]: https://developer.hashicorp.com/terraform
[juju-terraform-provider]: https://registry.terraform.io/providers/juju/juju/latest/docs

## Project and community
```

- [ ] **Step 3: Verify the README renders sensibly**

```bash
grep -n "Terraform module" README.md
```

Expected: shows the new heading, positioned before `## Project and community`.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/test_terraform_modules.yaml README.md
git commit -m "ci: add terraform module tests workflow and README section"
```

---

## Task 8: Final repo-wide validation pass

**Files:**
- None created/modified (validation only).

**Interfaces:**
- None.

- [ ] **Step 1: Re-run structural validation across both modules from the repo root**

```bash
cd /home/daniel.nguyen@canonical.com/bingo
for dir in terraform terraform/product; do
  echo "=== $dir ===";
  (cd "$dir" && terraform fmt -check && terraform init -backend=false -upgrade && terraform validate)
done
tflint --chdir=terraform --recursive
tflint --chdir=terraform/product --recursive
```

Expected: no `fmt` diffs, both `terraform validate` calls print `Success! The
configuration is valid.`, both `tflint` calls report no errors.

- [ ] **Step 2: Confirm no stray `.terraform` directories or lock files are tracked**

```bash
git status --porcelain | grep -E '\.terraform|\.terraform\.lock\.hcl' || echo "clean"
```

Expected: prints `clean` (these are build artifacts and should not be
committed — add a `.gitignore` entry if any are found).

- [ ] **Step 3: If Step 2 found stray files, add a `.gitignore` entry and remove them**

```bash
cat >> .gitignore <<'EOF'

# Terraform
**/.terraform/
**/.terraform.lock.hcl
EOF
git rm -r --cached --ignore-unmatch terraform/.terraform terraform/product/.terraform terraform/tests/setup/.terraform terraform/product/tests/setup/.terraform 2>/dev/null || true
git add .gitignore
git commit -m "chore: ignore terraform build artifacts"
```

(Skip this step entirely if Step 2 already printed `clean`.)

- [ ] **Step 4: Final review — confirm the full file tree matches the design**

```bash
find terraform -type f | sort
```

Expected output (order may vary slightly):

```
terraform/.tflint.hcl
terraform/README.md
terraform/main.tf
terraform/outputs.tf
terraform/product/.tflint.hcl
terraform/product/README.md
terraform/product/main.tf
terraform/product/outputs.tf
terraform/product/tests/main.tftest.hcl
terraform/product/tests/setup/main.tf
terraform/product/variables.tf
terraform/product/versions.tf
terraform/tests/main.tftest.hcl
terraform/tests/setup/main.tf
terraform/variables.tf
terraform/versions.tf
```
