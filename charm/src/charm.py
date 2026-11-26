#!/usr/bin/env python3
# Copyright 2026 Canonical Ltd.
# See LICENSE file for licensing details.

"""BingoCharm — thin paas_charm.go.Charm subclass for bingo."""

import typing

import ops
import paas_charm.go


class BingoCharm(paas_charm.go.Charm):
    """Charm for the bingo Go application.

    Inherits all reconciliation logic from paas_charm.go.Charm.  The go-framework
    charm type manages the pebble service layer, PostgreSQL relation, ingress, logging,
    and metrics wiring automatically.  This class exists so we can customise behaviour
    in the future without modifying the upstream library.
    """

    @property
    def _base_url(self) -> str:
        """Return the base_url for the service.

        paas_charm.go.Charm._base_url always resolves to the ingress URL (falling
        back to the in-cluster K8s service URL), ignoring any user-supplied value.
        The bingo-specific `base-url` config option exists precisely so operators
        can override the externally-visible link text used in generated paste
        URLs (e.g. a public DNS name fronting the ingress), so it must take
        priority here. Without this override, `juju config bingo base-url=...`
        would be silently ignored whenever an ingress relation is present.
        """
        base_url = typing.cast(str, self.config.get("base-url", ""))
        if base_url:
            return base_url
        return super()._base_url


if __name__ == "__main__":  # pragma: nocover
    ops.main(BingoCharm)
