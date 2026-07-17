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
