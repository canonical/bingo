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
