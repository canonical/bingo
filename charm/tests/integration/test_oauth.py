# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Integration test for BingoCharm's oauth relation with the identity platform.

Runs as its own spread task (own fresh MicroK8s/Juju model) so its runtime
doesn't accumulate on top of the other integration suites -- see _base.py.
"""

import logging

import jubilant

from ._base import deploy_base

logger = logging.getLogger(__name__)

_HYDRA_CHANNEL = "latest/edge"
_KRATOS_CHANNEL = "latest/edge"
_LOGIN_UI_CHANNEL = "latest/edge"


def _deploy_identity_bundle(juju: jubilant.Juju) -> None:
    """Deploy a minimal Canonical identity bundle (hydra + kratos + login-ui).

    Reuses the "postgresql" app already deployed for bingo:postgresql, since
    postgresql-k8s supports multiple simultaneous client relations.
    """
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


def test_oauth_integration(
    charm_path: str, juju: jubilant.Juju, resource_images: dict[str, str]
) -> None:
    """Integrate bingo with the identity platform over the oauth relation.

    arrange: deploy bingo + hydra/kratos/login-ui, wired to a shared postgresql app.
    act: integrate bingo:oauth with hydra:oauth.
    assert: both bingo and hydra settle to active with the relation established.
    """
    deploy_base(charm_path, juju, resource_images, with_traefik=True)
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
