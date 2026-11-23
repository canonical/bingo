# Changelog

## Unreleased

### Changed

- Upgraded the charm to `paas-charm` 2.0 and moved the charm and the rock to the Ubuntu 26.04
  LTS base. The charm now runs as a non-root user and is built with the `uv` plugin.
- The application secret key is regenerated on upgrade, which signs out every OIDC session,
  unless you supply your own key through the `app-secret-key` Juju secret option. See
  {ref}`how_to_upgrade`.
- Refreshing requires `--base ubuntu@26.04` and supplying the rebuilt contents of the existing `app-image` OCI resource.
