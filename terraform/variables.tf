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
