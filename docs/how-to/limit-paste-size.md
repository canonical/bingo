---
myst:
  html_meta:
    "description lang=en": "Learn how to cap the maximum paste content size accepted by the bingo charm."
---

(how_to_limit_paste_size)=

# How to limit paste size

Capping the maximum size of paste content accepted by bingo enables you to protect the
application and its storage backend from abuse or accidental overload, keep resource usage
predictable, and guard against denial-of-service attempts that submit excessively large
pastes.

## Limit paste size

This charm exposes the `max-paste-size-bytes` configuration option to specify the maximum paste
content size, in bytes, that the application will accept. The default is `5242880` (5 MiB).

For example, lower the limit to 1 MiB:

```
juju config bingo max-paste-size-bytes=1048576
```

Verify the configuration was applied:

```
juju config bingo max-paste-size-bytes
```

This command should return `1048576`.

## Verify

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
