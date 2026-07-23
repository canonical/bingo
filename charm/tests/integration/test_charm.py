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
from minio import Minio

logger = logging.getLogger(__name__)

# Revisions pinned for reproducible CI runs; bump deliberately when needed.
_HYDRA_CHANNEL = "latest/edge"
_KRATOS_CHANNEL = "latest/edge"
_LOGIN_UI_CHANNEL = "latest/edge"
_TEMPO_WORKER_CHANNEL = "2/edge"
_TEMPO_COORDINATOR_CHANNEL = "2/edge"
_MINIO_CHANNEL = "edge"
_S3_INTEGRATOR_CHANNEL = "edge"


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
    # traefik-k8s routes ingress-per-app requests under a
    # "{model}-{app}" path prefix (see TraefikIngressCharm._get_prefix),
    # not just the bare app name, so the prefix must be built dynamically.
    prefix = f"{status.model.name}-bingo"
    url = f"http://{ip}/{prefix}/api/v1/healthz"
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
    prefix = f"{status.model.name}-bingo"
    url = f"http://{ip}/{prefix}/api/v1/pastes"
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


def _deploy_identity_bundle(juju: jubilant.Juju) -> None:
    """Deploy a minimal Canonical identity bundle (hydra + kratos + login-ui).

    Reuses the "postgresql" app already deployed for bingo:postgresql, since
    postgresql-k8s supports multiple simultaneous client relations.
    """
    status = juju.status()
    if "hydra" in status.apps:
        logger.info("identity bundle already deployed")
        return

    # dev=True disables hydra's HTTPS-only public ingress requirement; the test's
    # traefik-k8s doesn't terminate TLS, so hydra would otherwise stay "blocked".
    juju.deploy("hydra", channel=_HYDRA_CHANNEL, trust=True, config={"dev": True})
    juju.deploy("kratos", channel=_KRATOS_CHANNEL, trust=True)
    juju.deploy("identity-platform-login-ui-operator", channel=_LOGIN_UI_CHANNEL, trust=True)

    juju.integrate("hydra:pg-database", "postgresql:database")
    juju.integrate("kratos:pg-database", "postgresql:database")
    juju.integrate(
        "hydra:hydra-endpoint-info", "identity-platform-login-ui-operator:hydra-endpoint-info"
    )
    juju.integrate("hydra:hydra-endpoint-info", "kratos:hydra-endpoint-info")
    juju.integrate("kratos:kratos-info", "identity-platform-login-ui-operator:kratos-info")
    juju.integrate(
        "hydra:ui-endpoint-info", "identity-platform-login-ui-operator:ui-endpoint-info"
    )
    juju.integrate(
        "kratos:ui-endpoint-info", "identity-platform-login-ui-operator:ui-endpoint-info"
    )
    # hydra requires a public-route relation to a traefik-route provider or it stays
    # blocked ("Missing required relation with public-route"); reuse the traefik app
    # already deployed for bingo's own ingress relation.
    juju.integrate("hydra:public-route", "traefik:traefik-route")

    juju.wait(
        lambda status: jubilant.all_active(status, "hydra", "kratos"),
        timeout=900,
        delay=10,
    )
    logger.info("identity bundle is active")


def test_oauth_integration(juju: jubilant.Juju) -> None:
    """Integrate bingo with the identity platform over the oauth relation.

    arrange: deploy hydra/kratos/login-ui, wired to the shared postgresql app.
    act: integrate bingo:oauth with hydra:oauth.
    assert: both bingo and hydra settle to active with the relation established.
    """
    _deploy_identity_bundle(juju)

    juju.integrate("bingo:oauth", "hydra:oauth")
    juju.wait(
        lambda status: jubilant.all_active(status, "bingo", "hydra"),
        timeout=900,
        delay=10,
    )

    status = juju.status()
    assert status.apps["bingo"].relations.get("oauth"), "bingo:oauth relation not established"
    logger.info("bingo:oauth relation is active")


def _deploy_tracing_stack(juju: jubilant.Juju) -> None:
    """Deploy a minio-backed Tempo (tracing) stack for testing.

    Mirrors the pattern used by canonical/paas-charm and canonical/tempo-operators'
    deploy_minio.py: minio + s3-integrator provide the object storage backend
    tempo-coordinator-k8s requires.
    """
    status = juju.status()
    if "tempo" in status.apps:
        logger.info("tracing stack already deployed")
        return

    access_key, secret_key, bucket = "accesskey", "secretkey", "tempo"

    juju.deploy(
        "minio",
        channel=_MINIO_CHANNEL,
        trust=True,
        config={"access-key": access_key, "secret-key": secret_key},
    )
    juju.deploy("s3-integrator", app="s3-integrator", channel=_S3_INTEGRATOR_CHANNEL)
    juju.deploy("tempo-worker-k8s", app="tempo-worker", channel=_TEMPO_WORKER_CHANNEL, trust=True)
    juju.deploy(
        "tempo-coordinator-k8s", app="tempo", channel=_TEMPO_COORDINATOR_CHANNEL, trust=True
    )

    juju.wait(
        lambda status: (
            jubilant.all_active(status, "minio") and jubilant.all_blocked(status, "s3-integrator")
        ),
        timeout=600,
        delay=10,
    )

    status = juju.status()
    minio_addr = status.apps["minio"].units["minio/0"].address
    mc_client = Minio(
        f"{minio_addr}:9000",
        access_key=access_key,
        secret_key=secret_key,
        secure=False,
    )
    if not mc_client.bucket_exists(bucket):
        mc_client.make_bucket(bucket)

    model_name = status.model.name
    minio_hostname = f"minio-0.minio-endpoints.{model_name}.svc.cluster.local"
    juju.config("s3-integrator", {"endpoint": f"{minio_hostname}:9000", "bucket": bucket})
    juju.run(
        "s3-integrator/leader",
        "sync-s3-credentials",
        {"access-key": access_key, "secret-key": secret_key},
    )

    juju.integrate("tempo:s3", "s3-integrator:s3-credentials")
    juju.integrate("tempo:tempo-cluster", "tempo-worker:tempo-cluster")

    juju.wait(
        lambda status: jubilant.all_active(status, "tempo", "tempo-worker"),
        timeout=600,
        delay=10,
    )
    logger.info("tracing stack is active")


def test_tracing_integration(juju: jubilant.Juju) -> None:
    """Integrate bingo with Tempo over the tracing relation.

    arrange: deploy a minio-backed tempo-coordinator-k8s/tempo-worker-k8s stack.
    act: integrate bingo:tracing with tempo:tracing.
    assert: both bingo and tempo settle to active with the relation established.
    """
    _deploy_tracing_stack(juju)

    juju.integrate("bingo:tracing", "tempo:tracing")
    juju.wait(
        lambda status: jubilant.all_active(status, "bingo", "tempo"),
        timeout=600,
        delay=10,
    )

    status = juju.status()
    assert status.apps["bingo"].relations.get("tracing"), "bingo:tracing relation not established"
    logger.info("bingo:tracing relation is active")
