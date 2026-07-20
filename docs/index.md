---
myst:
  html_meta:
    "description lang=en": "Discover the bingo charm, a Juju operator that deploys and manages bingo, a Go pastebin application."
---

(index)=

# bingo operator

A [Juju](https://juju.is/) [charm](https://documentation.ubuntu.com/juju/3.6/reference/charm/)
deploying and managing [bingo](https://github.com/canonical/bingo) on Kubernetes.
bingo is a Go pastebin application that replaces
[paste.canonical.com](https://paste.canonical.com).

Like any Juju charm, this charm supports one-line deployment, configuration, integration,
scaling, and more. For bingo, this includes:

* Paste creation and retrieval with configurable expiry
* Optional OIDC authentication via the Canonical Identity Platform
* Integration with PostgreSQL for persistent storage
* Ingress support via Traefik
* Observability integrations (Prometheus, Grafana, Loki)

The bingo charm allows for deployment on many different Kubernetes platforms,
from [MicroK8s](https://microk8s.io/) to
[Charmed Kubernetes](https://ubuntu.com/kubernetes) to public cloud Kubernetes offerings.

This charm will make operating bingo straightforward for DevOps or SRE teams through Juju's
clean interface.

## How this documentation is organized

This documentation uses the [Diátaxis documentation structure](https://diataxis.fr/).

- The {ref}`Tutorial <tutorial_index>` takes you step-by-step through a basic deployment of
  the bingo charm.
- {ref}`How-to guides <how_to_index>` assume you have basic familiarity with the bingo
  charm. Learn more about setting up, using, maintaining, and contributing to this charm.
- {ref}`Reference <reference_index>` provides a guide to actions, configurations,
  relations, and other technical details.
- {ref}`Explanation <explanation_index>` includes topic overviews, background and context
  and detailed discussion.
- {ref}`Release notes <release_notes_index>` holds all the release notes for the charm,
  including any system or upgrade requirements.

### Contributing to this documentation

Documentation is an important part of this project, and we take the same open-source
approach to the documentation as the code. As such, we welcome community contributions,
suggestions, and constructive feedback on our documentation.
See {ref}`How to contribute <how_to_contribute>` for more information.

If there's a particular area of documentation that you'd like to see that's missing,
please [file a bug](https://github.com/canonical/bingo/issues).

## Project and community

The bingo Operator is a member of the Ubuntu family. It's an open-source project that
warmly welcomes community projects, contributions, suggestions, fixes, and constructive
feedback.

### Governance and policies

- [Code of conduct](https://ubuntu.com/community/code-of-conduct)

### Get involved

- [Get support](https://discourse.charmhub.io/)
- [Join our online chat](https://matrix.to/#/#charmhub-charmdev:ubuntu.com)
- {ref}`Contribute <how_to_contribute>`

### Releases

- {ref}`Release notes <release_notes_index>`

Thinking about using the bingo Operator for your next project?
[Get in touch](https://matrix.to/#/#charmhub-charmdev:ubuntu.com)!

```{toctree}
:hidden:
:maxdepth: 1

Tutorial <tutorial/index>
how-to/index
reference/index
explanation/index
release-notes/index
```
