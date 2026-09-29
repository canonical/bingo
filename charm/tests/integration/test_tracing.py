# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Integration test for BingoCharm's tracing relation with Tempo.

Runs as its own spread task (own fresh MicroK8s/Juju model) so its runtime
doesn't accumulate on top of the other integration suites -- see _base.py.
"""

import logging

import jubilant
from minio import Minio

from ._base import deploy_base

logger = logging.getLogger(__name__)

_TEMPO_WORKER_CHANNEL = "2/edge"
_TEMPO_COORDINATOR_CHANNEL = "2/edge"
_MINIO_CHANNEL = "edge"
_S3_INTEGRATOR_CHANNEL = "edge"


def _deploy_tracing_stack(juju: jubilant.Juju) -> None:
    """Deploy a minio-backed Tempo (tracing) stack for testing.

    Mirrors the pattern used by canonical/paas-charm and canonical/tempo-operators'
    deploy_minio.py: minio + s3-integrator provide the object storage backend
    tempo-coordinator-k8s requires.
    """
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


def test_tracing_integration(
    charm_path: str, juju: jubilant.Juju, resource_images: dict[str, str]
) -> None:
    """Integrate bingo with Tempo over the tracing relation.

    arrange: deploy bingo and a minio-backed tempo-coordinator-k8s/tempo-worker-k8s stack.
    act: integrate bingo:tracing with tempo:tracing.
    assert: both bingo and tempo settle to active with the relation established.
    """
    deploy_base(charm_path, juju, resource_images)
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
