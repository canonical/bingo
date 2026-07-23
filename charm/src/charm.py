#!/usr/bin/env python3
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""BingoCharm — thin paas_charm.go.Charm subclass for bingo."""

import typing

import ops
import paas_charm.go

# The Go app's OIDC callback route is hardcoded to /auth/callback and does
# not read the oauth-redirect-path config. paas_charm requires the config
# key to exist so its relation wiring defaults to the right value, but the
# value itself must never change — otherwise the identity provider is told
# to redirect to a path the app doesn't serve, silently breaking login.
_REQUIRED_OAUTH_REDIRECT_PATH = "/auth/callback"


class BingoCharm(paas_charm.go.Charm):
    """Charm for the bingo Go application.

    Inherits all reconciliation logic from paas_charm.go.Charm.  The go-framework
    charm type manages the pebble service layer, PostgreSQL relation, ingress, logging,
    and metrics wiring automatically.  This class exists so we can customise behaviour
    in the future without modifying the upstream library.
    """

    def __init__(self, *args: typing.Any) -> None:
        """Initialise the BingoCharm."""
        super().__init__(*args)
        # Observed after paas_charm's own config-changed handler (registered
        # above in super().__init__()), so this guard has the final say on
        # unit status when oauth-redirect-path is misconfigured.
        self.framework.observe(self.on.config_changed, self._on_oauth_redirect_path_changed)

    def _on_oauth_redirect_path_changed(self, _: ops.EventBase) -> None:
        """Block the charm if oauth-redirect-path is changed from its required value.

        Args:
            _: The config-changed event (unused).
        """
        redirect_path = self.config.get("oauth-redirect-path")
        if redirect_path != _REQUIRED_OAUTH_REDIRECT_PATH:
            self.unit.status = ops.BlockedStatus(
                f"oauth-redirect-path must be {_REQUIRED_OAUTH_REDIRECT_PATH!r}; "
                "the app's callback route is fixed and not configurable"
            )


if __name__ == "__main__":  # pragma: nocover
    ops.main(BingoCharm)
