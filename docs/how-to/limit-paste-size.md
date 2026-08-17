---
myst:
  html_meta:
    "description lang=en": "Learn how to cap the maximum paste content size accepted by the bingo charm."
---

(how_to_limit_paste_size)=

# How to limit paste size

This guide provides instructions for capping the maximum size of paste content accepted by
bingo.

## Prerequisites

Deploy the bingo charm.

```
juju deploy bingo
```

## Limit paste size

This charm exposes the `max-paste-size-bytes` configuration option to specify the maximum paste
content size, in bytes, that the application will accept. The default is `5242880` (5 MiB).

To lower the limit to, for example, 1 MiB:

```
juju config bingo max-paste-size-bytes=1048576
```

Verify the configuration was applied:

```
juju config bingo max-paste-size-bytes
```

```{terminal}
:output-only:

1048576
```

Confirm the limit is enforced by submitting a paste larger than the configured size and checking
that the request is rejected. Replace `<bingo-address>` with the address you use to reach bingo,
for example the ingress address shown in `juju status`:

```
printf '{"content":"%s"}' "$(head -c 1500000 /dev/urandom | base64 | tr -d '\n')" > /tmp/paste.json
curl -s -X POST http://<bingo-address>/api/v1/pastes \
  -H 'Content-Type: application/json' \
  --data-binary @/tmp/paste.json -w '\n%{http_code}\n'
```

The request should be rejected with an HTTP `413 Payload Too Large` response, while
a paste under the configured limit succeeds.

Alternatively, open bingo in a web browser at `http://<bingo-address>`, paste in content larger
than the configured limit, and submit it. The application should display an error indicating the
content is too large, rather than creating the paste.
