---
myst:
  html_meta:
    "description lang=en": "Potential security risks in the bingo charm and best practices to avoid them."
---

(explanation_security)=

# Security

Understanding the security risks in the `bingo` charm, and following the accompanying best
practices, helps you protect your deployment against data loss, unauthorized access to pastes,
and service disruption.

## Outdated software

Outdated software components, such as the upstream workload, can introduce exploitable security
vulnerabilities.

### Best practices

- Regularly update the charm revision to include the latest charm components. Updates include
  security fixes from the dependencies and the workload, as the charm dependencies are regularly
  updated.
- Regularly update Juju to the latest version to include security fixes.
- Deploy observability, like the
  [Canonical Observability Stack](https://documentation.ubuntu.com/observability/latest/how-to/deploy-and-manage/install/),
  to detect any unusual behaviors.

## Loss of data

Pastes live only in PostgreSQL: `bingo` has no S3 or other object storage option, so any loss or
corruption of the database results in permanent loss of all pastes.

### Best practices

- Use a dedicated Charmed PostgreSQL deployment and regularly back up the database through the
  charm's [backup action](https://canonical.com/data/postgresql/docs/latest/how-to/back-up-and-restore/create-a-backup/).

<!-- vale Canonical.007-Headings-sentence-case = NO -->
<!-- DOS is an acronym -->

## Denial-of-service (DOS) attacks

<!-- vale Canonical.007-Headings-sentence-case = YES-->

Malicious attackers can overwhelm `bingo` with requests, making the application unresponsive to
legitimate users. `bingo` has no built-in rate limiting; it relies entirely on the surrounding
infrastructure to control request volume.

### Best practices

- Deploy an ingress that can limit the number of requests from users, such as
  [Traefik](https://charmhub.io/traefik-k8s) or [HAProxy](https://charmhub.io/haproxy), and
  configure request-rate throttling there.
- Set the {ref}`max-paste-size-bytes <how_to_limit_paste_size>` configuration to bound the
  resource cost of a single paste. This limits per-paste resource use, but does not address
  request volume.

## Unencrypted traffic

`bingo` does not terminate TLS itself: it always serves plain HTTP and relies entirely on the
charm's ingress integration (Traefik) to provide encryption. Unless TLS is terminated at the
ingress, traffic between `bingo` and its clients is unencrypted, risking eavesdropping and
tampering.

`bingo`'s session and CSRF cookies are always set with `Secure: true`, so browsers
silently drop them over plain HTTP and authentication breaks without any visible error.

### Best practices

- Terminate TLS at the ingress, for example by integrating [Traefik](https://charmhub.io/traefik-k8s)
  with a TLS provider or cert-manager. This is required both for usability and to protect session
  cookies from interception.

## Confidentiality of pastes

Confidentiality of pastes relies on OIDC authentication (when configured) and link secrecy.

- Without OIDC configured, there is no authentication at all: anyone who can reach `bingo` can
  create or read any paste. All pastes created this way are anonymous (no owner), so none of
  them can be explicitly deleted; they are only removed once they expire.
- With OIDC enabled, the entire application is gated behind login, so only authenticated users
  can reach it. Any authenticated user with access to a paste's key can view that paste.
  Confidentiality among authenticated users still depends on keeping paste links secret.

### Best practices

- Enable {ref}`OIDC authentication <how_to_configure_oidc_login>` to restrict who can reach
  `bingo`.
- Recommend short expiry values (`1d` or `1w`) over long-lived ones (`1y`) when creating pastes,
  since paste services routinely receive accidentally shared secrets or credentials.
- Authenticated paste owners can delete their own pastes once they are no longer needed, rather
  than waiting for expiry.

## Secret key hygiene

`bingo`'s shared application secret key signs OIDC session cookies. A leaked or long-lived secret
key allows an attacker to forge valid session cookies.

### Best practices

- {ref}`Rotate the secret key <how_to_rotate_secret_key>` immediately if a breach is suspected,
  in addition to following routine credential-hygiene practices.
