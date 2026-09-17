# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Shared setup helpers for bingo charm integration test modules.

Each ``test_*.py`` module in this package is discovered by opcli as its own
spread task, running against its own fresh MicroK8s/Juju model (see
``spread.yaml``). Since Juju models aren't shared across modules, every module
that needs the bingo charm deployed calls :func:`deploy_base` itself rather
than relying on state from another module.

Not a test module itself (doesn't match the ``test_*.py`` discovery pattern),
so opcli/spread won't try to generate a task from it.
"""

from __future__ import annotations

import logging

import jubilant

logger = logging.getLogger(__name__)


def deploy_base(
    charm_path: str,
    juju: jubilant.Juju,
    resource_images: dict[str, str],
    *,
    with_traefik: bool = False,
) -> None:
    """Deploy bingo + postgresql (bingo requires postgresql to reach active).

    Optionally also deploys traefik-k8s and wires up bingo:ingress, for tests
    that need ingress (healthz/paste HTTP checks, or hydra's public-route).
    """
    juju.deploy(charm_path, app="bingo", resources=resource_images, trust=True)

    juju.deploy("postgresql-k8s", app="postgresql", channel="14/stable", trust=True)
    juju.integrate("bingo:postgresql", "postgresql:database")

    if with_traefik:
        juju.deploy("traefik-k8s", app="traefik", channel="latest/stable", trust=True)
        juju.integrate("bingo:ingress", "traefik:ingress")

    juju.wait(jubilant.all_active, timeout=600)
    logger.info("Base applications reached active status.")
