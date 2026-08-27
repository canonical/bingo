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
