# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Integration test for BingoCharm's grafana-dashboard relation with Grafana.

Runs as its own spread task (own fresh MicroK8s/Juju model) so its runtime
doesn't accumulate on top of the other integration suites -- see _base.py.
"""

import logging

import jubilant

from ._base import deploy_base

logger = logging.getLogger(__name__)

_GRAFANA_CHANNEL = "2/stable"


def test_grafana_dashboard_integration(
    charm_path: str, juju: jubilant.Juju, resource_images: dict[str, str]
) -> None:
    """Integrate bingo with Grafana over the grafana-dashboard relation.

    arrange: deploy bingo and grafana-k8s.
    act: integrate bingo:grafana-dashboard with grafana:grafana-dashboard.
    assert: both bingo and grafana settle to active with the relation established.
    """
    deploy_base(charm_path, juju, resource_images)

    juju.deploy("grafana-k8s", app="grafana", channel=_GRAFANA_CHANNEL, trust=True)
    juju.wait(lambda status: jubilant.all_active(status, "grafana"), timeout=600, delay=10)

    juju.integrate("bingo:grafana-dashboard", "grafana:grafana-dashboard")
    juju.wait(
        lambda status: jubilant.all_active(status, "bingo", "grafana"),
        timeout=600,
        delay=10,
    )

    status = juju.status()
    assert status.apps["bingo"].relations.get("grafana-dashboard"), (
        "bingo:grafana-dashboard relation not established"
    )
    logger.info("bingo:grafana-dashboard relation is active")
