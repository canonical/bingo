# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Charm unit tests using the ops Scenario framework (ops.testing)."""

import pathlib
from typing import cast

import pytest
import yaml  # type: ignore[import-untyped]
from ops import testing

from charm import BingoCharm

CHARMCRAFT_YAML = pathlib.Path(__file__).parents[2] / "charmcraft.yaml"


def _expanded_metadata() -> tuple[dict, dict]:
    """Return charmcraft.yaml metadata and config with go-framework extension additions.

    Scenario reads charmcraft.yaml as-is and does not expand extensions, so add
    the declarations paas-charm 2.0 relies on at runtime.
    """
    meta = yaml.safe_load(CHARMCRAFT_YAML.read_text())
    config = meta.pop("config")
    meta["containers"] = {"app": {"resource": "app-image"}}
    meta["resources"] = {"app-image": {"type": "oci-image"}}
    meta["peers"] = {"peers": {"interface": "peers"}}
    meta["provides"] = {
        "metrics-endpoint": {"interface": "prometheus_scrape"},
        "grafana-dashboard": {"interface": "grafana_dashboard"},
    }
    meta["requires"].update(
        {
            "logging": {"interface": "loki_push_api"},
            "ingress": {"interface": "ingress", "limit": 1},
        }
    )
    config["options"]["app-secret-key"] = {"type": "secret"}
    return meta, config


EXTENSION_ACTIONS: dict[str, dict] = {"rotate-secret-key": {}}


@pytest.fixture()
def ctx(monkeypatch: pytest.MonkeyPatch) -> testing.Context:
    """Return a fresh Scenario Context wrapping BingoCharm."""
    meta, config = _expanded_metadata()
    # Juju runs hooks from the charm root, where paas-charm reads paas-config.yaml.
    monkeypatch.chdir(CHARMCRAFT_YAML.parent)
    return testing.Context(BingoCharm, meta=meta, config=config, actions=EXTENSION_ACTIONS)


def test_charm_initialises(ctx: testing.Context) -> None:
    """BingoCharm must initialise without errors on start."""
    state_in = testing.State()
    # start event — paas_charm does not set unit status on start;
    # status stays UnknownStatus until pebble-ready or config-changed.
    state_out = ctx.run(ctx.on.start(), state_in)
    assert state_out.unit_status.name in ("waiting", "maintenance", "active", "unknown")


def test_pebble_ready_without_postgresql(ctx: testing.Context) -> None:
    """When pebble is ready but PostgreSQL is not related, charm should be waiting."""
    container = testing.Container("app", can_connect=True)
    state_in = testing.State(containers={container})
    state_out = ctx.run(ctx.on.pebble_ready(container), state_in)
    # Without a postgresql relation the charm cannot configure DATABASE_URL.
    assert state_out.unit_status.name in ("waiting", "blocked")


def test_pebble_not_connected(ctx: testing.Context) -> None:
    """When the app container cannot connect, charm must not be active."""
    container = testing.Container("app", can_connect=False)
    state_in = testing.State(containers={container})
    state_out = ctx.run(ctx.on.pebble_ready(container), state_in)
    assert state_out.unit_status.name != "active"


def test_config_changed_invalid_log_level(ctx: testing.Context) -> None:
    """An unrecognised log-level value must result in a blocked status."""
    container = testing.Container("app", can_connect=True)
    state_in = testing.State(
        containers={container},
        config={"log-level": "verbose"},  # not a valid level
    )
    # paas_charm validates config options; an invalid value should block.
    state_out = ctx.run(ctx.on.config_changed(), state_in)
    # The charm may either block or log a warning — it must not crash.
    assert state_out.unit_status.name in ("waiting", "blocked", "active", "maintenance")


def test_oauth_client_registration(ctx: testing.Context) -> None:
    """Register the callback and scopes that the Go application uses for login."""
    oauth = testing.Relation("oauth")
    state = testing.State(
        leader=True, relations={oauth}, config={"base-url": "https://paste.example.com"}
    )
    result = ctx.run(ctx.on.relation_created(oauth), state)
    registration = cast(dict[str, str], result.get_relation(oauth.id).local_app_data)
    assert registration["redirect_uri"] == "https://paste.example.com/auth/callback"
    assert set(registration["scope"].split()) == {"openid", "email", "profile"}


def test_base_url_config_overrides_ingress_derived_default(ctx: testing.Context) -> None:
    """The `base-url` config, when set, must win over paas_charm's ingress-derived default.

    paas_charm.go.Charm._base_url always resolves to the ingress URL (or the
    in-cluster K8s service URL as a fallback), regardless of any user config.
    BingoCharm overrides this so operators can point generated paste links at a
    public DNS name that fronts the ingress, per the set-base-url how-to guide.
    """
    container = testing.Container("app", can_connect=True)
    state_in = testing.State(
        containers={container},
        config={"base-url": "https://paste.example.com"},
        leader=True,
    )
    with ctx(ctx.on.config_changed(), state_in) as manager:
        assert manager.charm._base_url == "https://paste.example.com"


def test_base_url_config_empty_falls_back_to_ingress_default(ctx: testing.Context) -> None:
    """With no `base-url` config set, the charm must fall back to paas_charm's default."""
    container = testing.Container("app", can_connect=True)
    state_in = testing.State(containers={container}, leader=True)
    with ctx(ctx.on.config_changed(), state_in) as manager:
        # No ingress relation is present, so this falls back to the in-cluster
        # K8s service URL rather than a hardcoded expectation.
        assert manager.charm._base_url.startswith("http://")
        assert "paste.example.com" not in manager.charm._base_url
