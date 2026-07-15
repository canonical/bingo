# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Integration tests for BingoCharm.

These tests run via jubilant + pytest-jubilant inside the charm-ci spread environment
(MicroK8s + Juju 3.6). They are NOT run during local development.

Usage (in spread environment):
    pytest tests/integration/ -v
    (charm_path/resource_images are supplied automatically by the pytest-opcli
    plugin from artifacts.build.yaml; see opcli.pytest_plugin.)
"""

import json
import logging
import urllib.request

import jubilant
import pytest

logger = logging.getLogger(__name__)


@pytest.mark.juju_setup
def test_build_and_deploy(
    charm_path: str, juju: jubilant.Juju, resource_images: dict[str, str]
) -> None:
    """Deploy the charm with the OCI rock."""
    juju.deploy(charm_path, app="bingo", resources=resource_images, trust=True)

    juju.deploy("postgresql-k8s", app="postgresql", channel="14/stable", trust=True)
    juju.integrate("bingo:postgresql", "postgresql:database")

    juju.deploy("traefik-k8s", app="traefik", channel="latest/stable", trust=True)
    juju.integrate("bingo:ingress", "traefik:ingress")

    juju.wait(jubilant.all_active, timeout=600)
    logger.info("All applications reached active status.")


def test_healthz_responds(juju: jubilant.Juju) -> None:
    """The /api/v1/healthz endpoint must return HTTP 200."""
    status = juju.status()
    traefik_app = status.apps.get("traefik")
    if not traefik_app:
        pytest.skip("Traefik not deployed; skipping healthz check")
    traefik_unit = next(iter(traefik_app.units.values()))
    ip = traefik_unit.address
    url = f"http://{ip}/bingo/api/v1/healthz"
    logger.info("Checking healthz at %s", url)
    with urllib.request.urlopen(url, timeout=10) as resp:
        assert resp.status == 200, f"Expected 200, got {resp.status}"


def test_paste_create_anonymous(juju: jubilant.Juju) -> None:
    """Anonymous paste creation must return 201 with a key."""
    status = juju.status()
    traefik_app = status.apps.get("traefik")
    if not traefik_app:
        pytest.skip("Traefik not deployed")
    traefik_unit = next(iter(traefik_app.units.values()))
    ip = traefik_unit.address
    url = f"http://{ip}/bingo/api/v1/pastes"
    payload = json.dumps(
        {
            "content": "hello from integration test",
            "language": "plaintext",
            "expires_in": "1d",
        }
    ).encode()
    req = urllib.request.Request(
        url,
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        assert resp.status == 201
        body = json.loads(resp.read())
        assert "key" in body
        assert len(body["key"]) >= 4
        logger.info("Created paste with key: %s", body["key"])
