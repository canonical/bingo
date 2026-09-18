# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Integration test for BingoCharm's metrics-endpoint relation with Prometheus.

Runs as its own spread task (own fresh MicroK8s/Juju model) so its runtime
doesn't accumulate on top of the other integration suites -- see _base.py.
"""

import logging

import jubilant

from ._base import deploy_base

logger = logging.getLogger(__name__)

_PROMETHEUS_CHANNEL = "2/stable"


def test_metrics_endpoint_integration(
    charm_path: str, juju: jubilant.Juju, resource_images: dict[str, str]
) -> None:
    """Integrate bingo with Prometheus over the metrics-endpoint relation.

    arrange: deploy bingo and prometheus-k8s.
    act: integrate bingo:metrics-endpoint with prometheus:metrics-endpoint.
    assert: both bingo and prometheus settle to active with the relation established.
    """
    deploy_base(charm_path, juju, resource_images)

    juju.deploy("prometheus-k8s", app="prometheus", channel=_PROMETHEUS_CHANNEL, trust=True)
    juju.wait(lambda status: jubilant.all_active(status, "prometheus"), timeout=600, delay=10)

    juju.integrate("bingo:metrics-endpoint", "prometheus:metrics-endpoint")
    juju.wait(
        lambda status: jubilant.all_active(status, "bingo", "prometheus"),
        timeout=600,
        delay=10,
    )

    status = juju.status()
    assert status.apps["bingo"].relations.get("metrics-endpoint"), (
        "bingo:metrics-endpoint relation not established"
    )
    logger.info("bingo:metrics-endpoint relation is active")
