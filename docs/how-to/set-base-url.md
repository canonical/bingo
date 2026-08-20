---
myst:
  html_meta:
    "description lang=en": "Learn how to configure the public base URL the bingo charm uses to generate paste links."
---

(how_to_set_base_url)=

# How to set the base URL

Configuring the public base URL that bingo uses when generating paste links enables you to
share pastes with correct, externally reachable links once the application is exposed
through ingress, rather than links pointing at an internal or unreachable address.

## Prerequisites

Deploy the bingo charm and, if you want pastes to be reachable externally, an ingress provider
such as [`traefik-k8s`](https://charmhub.io/traefik-k8s).

```
juju deploy bingo
juju deploy traefik-k8s --trust
juju integrate bingo:ingress traefik-k8s:ingress
```

## Set the base URL

This charm exposes the `base-url` configuration option to specify the public base URL used when
building shareable paste links, for example `https://paste.example.com`.

If Traefik routes to bingo using a path prefix (the default "path" routing mode used by
`traefik-k8s`, as in this guide), `base-url` **must include that same path prefix**. The app also
uses `base-url` to determine the path prefix injected into the `<base href>` tag of the served
frontend and into OIDC redirect URIs, both of which need to match the prefix Traefik actually
routes on.

Use the `show-proxied-endpoints` action to determine the path prefix:

```{terminal}
:copy:

juju run traefik-k8s/0 show-proxied-endpoints

proxied-endpoints: '{"traefik-k8s": {"url": "http://10.10.161.187"}, "bingo": {"url":
  "http://10.10.161.187/bingo-tutorial-bingo"}}'
```

Then set `base-url` to your desired public domain, followed by the same path (`/bingo-tutorial-bingo`
in this example):

```
juju config bingo base-url=https://paste.example.com/bingo-tutorial-bingo
```

Verify the configuration was applied:

```
juju config bingo base-url
```

This command should return the URL you previously provided, for example `https://paste.example.com/bingo-tutorial-bingo`.

```{caution}
Omitting the path prefix (for example, setting `base-url=https://paste.example.com` when Traefik
routes on `/bingo-tutorial-bingo`) breaks the frontend: the browser will request static assets
from the domain root instead of under the routed path, resulting in a blank page.
```

## Verify

Create a paste against the `bingo` URL from the `show-proxied-endpoints` output above and confirm
the returned `url` field uses the configured base URL instead:

```
curl -s -X POST http://10.10.161.187/bingo-tutorial-bingo/api/v1/pastes \
  -H 'Content-Type: application/json' -d '{"content":"hello world"}'
```

The `url` and `raw_url` fields should be prefixed with the `base-url` value you configured,
confirming it took effect. 

```{terminal}
:output-only:

{"key":"abc123","url":"https://paste.example.com/bingo-tutorial-bingo/abc123","raw_url":"https://paste.example.com/bingo-tutorial-bingo/api/v1/pastes/abc123/raw", ...}
```

Also confirm the frontend still loads correctly by requesting the root path:

```
curl -s http://10.10.161.187/bingo-tutorial-bingo/ | grep -i "base href"
```

This command should return the correct path prefix, for example `<base href="/bingo-tutorial-bingo/">`.
