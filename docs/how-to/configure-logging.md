---
myst:
  html_meta:
    "description lang=en": "Learn how to adjust the log verbosity of the bingo charm's application."
---

(how_to_configure_logging)=

# How to configure logging

Adjusting the log verbosity of the bingo application enables you to troubleshoot issues in detail, monitor application health, and reduce log noise in stable deployments.

## Configure the log level

This charm exposes the `log-level` configuration option to control the verbosity of application
logs. Accepted values are `debug`, `info`, `warn`, and `error`. The default is `info`.

To increase verbosity for troubleshooting:

```
juju config bingo log-level=debug
```

Verify the configuration was applied:

```{terminal}
:copy:

juju config bingo log-level

debug
```

Changing this option restarts the workload with the new level applied. The option raises or lowers
the minimum severity of what gets logged: `debug` logs the most detail, while `warn` and `error`
suppress everything below that severity. Logs are written as JSON lines to stdout, with each
line's `"level"` field set to `INFO`, `DEBUG`, `WARN`, or `ERROR`.

## Verify

To verify the new level took effect, tail the workload's logs:

```
juju ssh --container app bingo/0 pebble logs go -f
```

The quickest way to see a visible change is to set a stricter level, such as `error`:

```
juju config bingo log-level=error
```

After the workload restarts, the usual `INFO` startup lines no longer appear:

```{terminal}
:output-only:

{"time":"2026-08-18T19:42:20Z","level":"INFO","msg":"shutting down"}
```

No further output follows the restart, confirming the `error` level suppressed the `INFO` lines.
