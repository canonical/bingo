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
