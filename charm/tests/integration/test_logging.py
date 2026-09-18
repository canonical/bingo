# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Integration test for BingoCharm's logging relation with Loki.

Runs as its own spread task (own fresh MicroK8s/Juju model) so its runtime
doesn't accumulate on top of the other integration suites -- see _base.py.
"""

import logging

import jubilant

from ._base import deploy_base

logger = logging.getLogger(__name__)

_LOKI_CHANNEL = "2/stable"


def test_logging_integration(
    charm_path: str, juju: jubilant.Juju, resource_images: dict[str, str]
) -> None:
    """Integrate bingo with Loki over the logging relation.

    arrange: deploy bingo and loki-k8s.
    act: integrate bingo:logging with loki:logging.
    assert: both bingo and loki settle to active with the relation established.
    """
    deploy_base(charm_path, juju, resource_images)

    juju.deploy("loki-k8s", app="loki", channel=_LOKI_CHANNEL, trust=True)
    juju.wait(lambda status: jubilant.all_active(status, "loki"), timeout=600, delay=10)

    juju.integrate("bingo:logging", "loki:logging")
    juju.wait(
        lambda status: jubilant.all_active(status, "bingo", "loki"),
        timeout=600,
        delay=10,
    )

    status = juju.status()
    assert status.apps["bingo"].relations.get("logging"), "bingo:logging relation not established"
    logger.info("bingo:logging relation is active")
