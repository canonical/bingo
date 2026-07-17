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
