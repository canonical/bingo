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
