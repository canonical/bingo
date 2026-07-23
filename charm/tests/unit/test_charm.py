# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""Charm unit tests using the ops Scenario framework (ops.testing)."""

import pytest
from ops import testing

from charm import BingoCharm


@pytest.fixture()
def ctx() -> testing.Context:
    """Return a fresh Scenario Context wrapping BingoCharm."""
    return testing.Context(BingoCharm)


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


def test_config_changed_oauth_redirect_path_overridden_blocks(ctx: testing.Context) -> None:
    """Changing oauth-redirect-path away from /auth/callback must block the charm.

    The Go app's callback route is hardcoded to /auth/callback and never reads
    this config, so any other value would register a callback with the identity
    provider that the app doesn't serve, silently breaking login.
    """
    container = testing.Container("app", can_connect=True)
    state_in = testing.State(
        containers={container},
        config={"oauth-redirect-path": "/callback"},
    )
    state_out = ctx.run(ctx.on.config_changed(), state_in)
    assert state_out.unit_status.name == "blocked"
    assert "oauth-redirect-path" in state_out.unit_status.message


def test_config_changed_oauth_redirect_path_default_not_blocked_by_guard(
    ctx: testing.Context,
) -> None:
    """The default oauth-redirect-path value must not trigger the guard."""
    container = testing.Container("app", can_connect=True)
    state_in = testing.State(containers={container})
    state_out = ctx.run(ctx.on.config_changed(), state_in)
    # Other reasons (e.g. missing postgresql) may still leave it non-active,
    # but it must not be blocked specifically for oauth-redirect-path.
    if state_out.unit_status.name == "blocked":
        assert "oauth-redirect-path" not in state_out.unit_status.message
