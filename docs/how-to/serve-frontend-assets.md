---
myst:
  html_meta:
    "description lang=en": "Learn how to configure where the bingo charm serves frontend assets from, or disable static file serving."
---

(how_to_serve_frontend_assets)=

# How to serve frontend assets

Configuring where bingo serves its built React frontend (SPA) assets from enables you to
accommodate a custom workload image whose build process bakes the assets into a
non-default directory, or to disable static file serving entirely when you host the
frontend separately (for example, on a CDN) and want bingo to serve only the API.

## Configure the web assets directory

This charm exposes the `web-dir` configuration option to specify the path, inside the workload
container, to the built frontend assets served as a single-page application (SPA). The default is
`/app/web/dist`.

Only change `web-dir` if your workload image bakes the built frontend assets into a directory
other than the default `/app/web/dist` (for example, if a custom build step copies the assets to
`/app/web/dist-custom`). Setting `web-dir` to a path that does not exist in the workload image
results in a `404` when requesting the root path, since the charm has nothing to serve from that
location.

To point at a different path baked into your workload image:

```
juju config bingo web-dir=/app/web/dist-custom
```

Verify the configuration was applied:

```
juju config bingo web-dir
```

This command should return the directory where the workload image lives.

Confirm the frontend is served correctly by requesting the application's root path. Replace
`<bingo-address>` with the address you use to reach bingo, for example the ingress address shown
in `juju status`:

```
curl -sI http://<bingo-address>/
```

The response should be a `200 OK` serving the SPA's `index.html`.

## Disable static file serving

If bingo should only serve the API (for example, when the frontend is hosted separately), set
`web-dir` to an empty string:

```
juju config bingo web-dir=""
```

Requesting the root path should no longer return the SPA:

```
curl -sI http://<bingo-address>/
```

The response should no longer serve `index.html` from the application (for example, a `404` for
unmatched frontend routes), while API endpoints continue to work.

Verify an API endpoint, such as the health check, still responds correctly:

```
curl -s http://<bingo-address>/api/v1/healthz
```

The command should respond with `{"status":"ok"}`.
