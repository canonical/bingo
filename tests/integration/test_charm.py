# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Integration tests for BingoCharm.

These tests run via pytest-operator inside the charm-ci spread environment
(MicroK8s + Juju 3.6). They are NOT run during local development.

Usage (in spread environment):
    pytest tests/integration/ -v --model testing
"""

import asyncio
import logging
import typing

import pytest
import pytest_asyncio
from juju.application import Application
from pytest_operator.plugin import OpsTest

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(scope="module")
def app_image(request: pytest.FixtureRequest) -> str:
    """OCI image path; provided by charm-ci via --app-image CLI option."""
    image = request.config.getoption("--app-image", default=None)
    if image is None:
        pytest.skip("--app-image not provided; skipping integration tests")
    return typing.cast(str, image)


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption(
        "--app-image",
        action="store",
        help="OCI image reference for the bingo app-image resource",
    )


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


@pytest.mark.abort_on_fail
async def test_build_and_deploy(ops_test: OpsTest, app_image: str) -> None:
    """Build the charm and deploy it with the OCI rock."""
    charm_path = await ops_test.build_charm(".")
    assert ops_test.model is not None

    app: Application = await ops_test.model.deploy(
        str(charm_path),
        application_name="bingo",
        resources={"app-image": app_image},
        trust=True,
    )

    # Deploy a PostgreSQL charm and relate it.
    await ops_test.model.deploy(
        "postgresql-k8s",
        application_name="postgresql",
        channel="14/stable",
        trust=True,
    )
    await ops_test.model.integrate("bingo:postgresql", "postgresql:database")

    # Deploy a traefik ingress.
    await ops_test.model.deploy(
        "traefik-k8s",
        application_name="traefik",
        channel="latest/stable",
        trust=True,
    )
    await ops_test.model.integrate("bingo:ingress", "traefik:ingress")

    await ops_test.model.wait_for_idle(
        apps=["bingo", "postgresql", "traefik"],
        status="active",
        timeout=600,
        raise_on_error=True,
    )
    logger.info("All applications reached active status.")


async def test_healthz_responds(ops_test: OpsTest) -> None:
    """The /api/v1/healthz endpoint must return HTTP 200."""
    import urllib.request

    assert ops_test.model is not None
    bingo_app = ops_test.model.applications.get("bingo")
    assert bingo_app is not None

    # Retrieve the ingress URL from the unit status message.
    unit = bingo_app.units[0]
    status_msg = unit.workload_status_message
    logger.info("Unit status: %s", status_msg)

    # Attempt healthz via the ingress address if available.
    traefik_app = ops_test.model.applications.get("traefik")
    if traefik_app:
        traefik_unit = traefik_app.units[0]
        # Traefik exposes its IP via status message or address.
        ip = await traefik_unit.get_public_address()
        url = f"http://{ip}/bingo/api/v1/healthz"
        logger.info("Checking healthz at %s", url)
        with urllib.request.urlopen(url, timeout=10) as resp:
            assert resp.status == 200, f"Expected 200, got {resp.status}"
    else:
        pytest.skip("Traefik not deployed; skipping healthz check")


async def test_paste_create_anonymous(ops_test: OpsTest) -> None:
    """Anonymous paste creation must return 201 with a key."""
    import json
    import urllib.request

    assert ops_test.model is not None
    traefik_app = ops_test.model.applications.get("traefik")
    if not traefik_app:
        pytest.skip("Traefik not deployed")

    ip = await traefik_app.units[0].get_public_address()
    url = f"http://{ip}/bingo/api/v1/pastes"
    payload = json.dumps({
        "content": "hello from integration test",
        "language": "plaintext",
        "expires_in": "1d",
    }).encode()
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
