# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

terraform {
  required_providers {
    juju = {
      source  = "juju/juju"
      version = ">= 2.2.0"
    }
  }
}

provider "juju" {}

resource "juju_model" "test_model" {
  name       = "tf-bingo-product-${formatdate("YYYYMMDDhhmmss", timestamp())}"
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
