# bingo Reference Terraform Module — Design

## Goal

Author a reusable [Juju Terraform Provider][juju-tf-provider] module for the bingo
charm so the IS team can deploy bingo (model, application, config, and relations) as
a building block in their own Terraform, without reading `charm/charmcraft.yaml` or
`charm/src/charm.py`. This follows the standard Canonical IS charm-Terraform-module
pattern already established for other charms (e.g. `canonical/mattermost-k8s-operator`
PR #70), which splits the module into a **base module** (application-only) and a
**product module** (model + bundled dependencies + integrations).

## Non-Goals

- Deploying or managing a full Canonical Observability Stack (COS) — `logging`,
  `metrics-endpoint`, and `grafana-dashboard` are wired only via externally-supplied
  cross-model offer URLs, not bundled charms.
- Deploying a production-grade HA tracing backend. `tempo-k8s` (the simple
  single-charm tracing provider) is deprecated; its replacement
  (`tempo-coordinator-k8s` + `tempo-worker-k8s` + S3 storage) is out of scope for this
  module. `tracing` is therefore also wired only via an externally-supplied offer URL.
- Running real `terraform apply` against a live Juju controller in this development
  environment (none is available here — no `juju`/`terraform` CLI, no k8s cloud).
  Validation here is structural (`fmt`, `init`, `validate`, `tflint`); real
  plan/apply validation happens in CI against a real k8s controller.
- Changing charm behavior, `charmcraft.yaml`, or application code.

## Architecture

### Base module — `terraform/`

Deploys the bingo `juju_application` only. Does not create a model and does not wire
any integrations — those are the caller's responsibility (directly, or via the
product module below).

```
terraform/
├── main.tf          # juju_application "bingo"
├── variables.tf     # app_name, model_uuid, base, channel, revision, config, constraints, units
├── outputs.tf       # app_name, requires{...}, provides{...}
├── versions.tf      # required_providers { juju = { source = "juju/juju" } }
├── README.md
├── .tflint.hcl
└── tests/
    ├── setup/main.tf      # ephemeral juju_model for CI-only testing
    └── main.tftest.hcl    # `terraform plan` run + assertions
```

**`main.tf`**
```hcl
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

**`variables.tf`**

| Variable | Type | Default | Notes |
|---|---|---|---|
| `app_name` | `string` | `"bingo"` | Juju application name |
| `model_uuid` | `string` | *(required)* | Model must already exist; created by the caller |
| `base` | `string` | `"ubuntu@24.04"` | Matches `charmcraft.yaml` |
| `channel` | `string` | `"latest/edge"` | Charmhub channel |
| `revision` | `number` | `null` | Pin a specific revision if set |
| `config` | `map(string)` | `{}` | Passthrough to charm config — keys match `charmcraft.yaml`, e.g. `"base-url"`, `"oauth-redirect-path"`, `"oauth-scopes"`, `"oauth-user-name-attribute"`, `"max-paste-size-bytes"`, `"log-level"`, `"web-dir"` |
| `constraints` | `string` | `""` | Juju constraints string |
| `units` | `number` | `1` | Unit count |

Using a generic `config` map (rather than one Terraform variable per charm config
key) avoids duplicating/staling charm config definitions in Terraform; the module
README links to the charm's config reference instead.

**`outputs.tf`**
```hcl
output "app_name" {
  value = juju_application.bingo.name
}

output "requires" {
  value = {
    postgresql = "postgresql"
    oauth      = "oauth"
    tracing    = "tracing"
    ingress    = "ingress"
    logging    = "logging"
  }
}

output "provides" {
  value = {
    metrics_endpoint  = "metrics-endpoint"
    grafana_dashboard = "grafana-dashboard"
  }
}
```

Note: `logging` (interface `loki_push_api`) is a `requires` relation, not
`provides` — confirmed against `charmcraft/extensions/app.py`'s go-framework
extension, which declares `requires: {logging, ingress}` and
`provides: {metrics-endpoint, grafana-dashboard}`. bingo requires a Loki
endpoint to push logs to, the same way it requires an ingress endpoint.

These maps let a caller build `juju_integration` blocks (`endpoint =
module.bingo.requires.postgresql`) without hardcoding or guessing endpoint names.

### Product module — `terraform/product/`

Deploys everything needed for a working bingo deployment end-to-end: owns the
model, instantiates the base module, optionally bundles dependency charms for the
relations that have a practical standalone provider, and wires all `juju_integration`
resources.

```
terraform/product/
├── main.tf
├── variables.tf
├── outputs.tf
├── versions.tf
├── README.md
└── tests/ (same setup/plan-test pattern as the base module)
```

**Model ownership**: `juju_model.this` (name/cloud/credential as variables) —
satisfies "deploys model+application" from the acceptance criteria.

**Bundled dependencies** (each behind a `deploy_*` bool, default `true`; set to
`false` to integrate with an existing app elsewhere instead):

| Relation | Bundled charm | Endpoint wired |
|---|---|---|
| `postgresql` (required) | `postgresql-k8s` | `bingo.postgresql` ↔ `postgresql-k8s.database` |
| `oauth` (optional) | `oauth-external-idp-integrator` | `bingo.oauth` ↔ `oauth-external-idp-integrator.oauth` |
| `ingress` (optional) | `traefik-k8s` | `bingo.ingress` ↔ `traefik-k8s.ingress` |

`oauth-external-idp-integrator`'s config (IdP issuer URL, client ID/secret, etc.) is
exposed as a variable object since it must describe a real external IdP — this is
the "expected external input" the ticket calls out for the IdP.

**Offer-URL-only integrations** (no bundled charm; each is an optional variable,
default `null`, skipped when unset): `tracing_offer_url`, `logging_offer_url`,
`metrics_offer_url`, `grafana_dashboard_offer_url`. When set, a `juju_integration`
is created against the remote cross-model offer instead of a local application.
These are the "expected external inputs" for tracing/observability infrastructure.

**`outputs.tf`**: `model_name`, `bingo.app_name` / `bingo.requires`,
`postgresql_app_name`, `oauth_app_name`, `ingress_app_name` — mirroring the shape
used by the mattermost product module's outputs.

## Documentation

- `terraform/README.md` and `terraform/product/README.md` document every variable
  (type, default, purpose), which relations are required vs. optional, which are
  bundled vs. offer-only, and the external inputs a caller must supply (reachable
  IdP details for oauth config; COS offer URLs for tracing/logging/metrics/
  grafana-dashboard if used). Include a usage example for both modules, matching
  the format of the reference `__charm_name__` Terraform module template.
- Root `README.md` gets a short new "Terraform module" section linking to
  `terraform/README.md`.

## Testing & Validation

**Local (this development environment — no live Juju controller available):**
- `terraform fmt -check -recursive`
- `terraform init -backend=false` + `terraform validate` for both `terraform/` and
  `terraform/product/`
- `tflint` for both modules

**CI (real validation, runs once merged — satisfies "terraform plan/apply
succeeds"):**
- Add `.github/workflows/test_terraform_modules.yaml` using the shared reusable
  workflow `canonical/operator-workflows/.github/workflows/terraform_modules_test.yaml`
  with `k8s-controller: true` and
  `terraform-directories: '["terraform", "terraform/product"]'`, matching the
  precedent set by `canonical/mattermost-k8s-operator`.
- Each module's `tests/main.tftest.hcl` runs a `setup` step (ephemeral
  `juju_model`) followed by a `basic_deploy` run (`command = plan`) asserting
  `output.app_name == "bingo"`.

## Open Risks / Follow-ups

- `oauth-external-idp-integrator` config schema should be double-checked against
  its current charm config options when implementing (may have changed since this
  design was written).
- If IS later needs a bundled tracing backend, revisit once
  `tempo-coordinator-k8s` HA topology (with S3 storage) is considered acceptable
  complexity for this module, or once a simpler drop-in replacement for
  `tempo-k8s` exists.

[juju-tf-provider]: https://registry.terraform.io/providers/juju/juju/latest/docs
