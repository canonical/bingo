# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

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
