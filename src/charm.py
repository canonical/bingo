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

    def __init__(self, *args: typing.Any) -> None:
        """Initialise the BingoCharm."""
        super().__init__(*args)


if __name__ == "__main__":  # pragma: nocover
    ops.main(BingoCharm)
