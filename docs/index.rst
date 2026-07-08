.. meta::
   :description: Discover the bingo charm, a Juju operator that deploys and manages bingo, a Go pastebin application.

.. vale Canonical.007-Headings-sentence-case = NO

.. _index:

bingo operator
==============

.. vale Canonical.007-Headings-sentence-case = YES

A `Juju <https://juju.is/>`_ `charm <https://documentation.ubuntu.com/juju/3.6/reference/charm/>`_
deploying and managing `bingo <https://github.com/canonical/bingo>`_ on Kubernetes.
bingo is a Go pastebin application that replaces
`paste.canonical.com <https://paste.canonical.com>`_.

Like any Juju charm, this charm supports one-line deployment, configuration, integration,
scaling, and more.
For bingo, this includes:

* Paste creation and retrieval with configurable expiry
* Optional OIDC authentication via the Canonical Identity Platform
* Integration with PostgreSQL for persistent storage
* Ingress support via Traefik
* Observability integrations (Prometheus, Grafana, Loki)

The bingo charm allows for deployment on many different Kubernetes platforms,
from `MicroK8s <https://microk8s.io/>`_ to
`Charmed Kubernetes <https://ubuntu.com/kubernetes>`_ to public cloud Kubernetes offerings.

This charm will make operating bingo straightforward for DevOps or
SRE teams through Juju's clean interface.

In this documentation
---------------------

.. TODO: Use the table below as a starting place.
   You don't need to include all of the rows if they're not relevant to the charm
   or if the docs don't exist. Use the vertical line symbol | to separate pages.
   
   When linking a how-to guide, use a verb to indicate an action/task. When
   linking reference or explanation material, use gerunds or nouns.

   Use "Get started" to highlight one or more tutorials. This row touches
   on the "point of entry" domain.

   For Deployment and Operations, place the most important/common use cases first.
   If there are no meaningful Day 0/1 operations, drop "Deployment" from the table.
   If there are no meaningful Day 2 operations, drop "Operations" from the table.
   These rows touch on the domain of "lifecycle".
   
   Use the "Product-specific feature" row to highlight any 
   major selling points of the charm -- what's the value proposition of this charm?

   Use the "Design" row to showcase architecture and design-related documentation
   for this charm. This row touches on the domain of "conceptual or stack layers".
   
   If possible, include a row that touches on the "quality" domain (security, performance).

   Another charm-specific row to consider is "Integrations", especially if the
   charm is meant to work in the context of a larger deployment. The "Integrations"
   row touches on the domain of "interfaces".

.. list-table::
    :header-rows: 1

    * - 
      - 
    * - Get started
      - :ref:`Guided tutorial <tutorial_index>` | :ref:`High-level deployment <reference_high_level_deployment>` 
    * - Deployment
      - Relevant how-to guides and reference pages (related to initial setup, configurations, and customization)
    * - Operations
      - Relevant how-to guides and reference pages (examples: integrate with COS, backup/restore, redeploy, upgrade)
    * - Product-specific feature
      - Relevant guides and pages
    * - Design
      - :ref:`Architecture <reference_charm_architecture>` | :ref:`Design <explanation_charm_design>`
    * - Security
      - :ref:`Overview <explanation_security>` | Relevant how-to guides | Relevant reference pages

How this documentation is organized
------------------------------------

This documentation uses the `Diátaxis documentation structure <https://diataxis.fr/>`_.

- The :ref:`Tutorial <tutorial_index>` takes you step-by-step through a basic deployment of the bingo charm.
- :ref:`How-to guides <how_to_index>` assume you have basic familiarity with the bingo charm. Learn more about setting up, using, maintaining, and contributing to this charm.
- :ref:`Reference <reference_index>` provides a guide to actions, configurations, relations, and other technical details.
- :ref:`Explanation <explanation_index>` includes topic overviews, background and context and detailed discussion.
- :ref:`Release notes <release_notes_index>` holds all the release notes for the charm, including any system or upgrade requirements.

Contributing to this documentation
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Documentation is an important part of this project, and we take the same open-source approach
to the documentation as the code. As such, we welcome community contributions, suggestions, and
constructive feedback on our documentation.
See :ref:`How to contribute <how_to_contribute>` for more information.

If there's a particular area of documentation that you'd like to see that's missing, please 
`file a bug <https://github.com/canonical/bingo/issues>`_.

Project and community
---------------------

The bingo Operator is a member of the Ubuntu family. It's an open-source project that warmly welcomes community 
projects, contributions, suggestions, fixes, and constructive feedback.

Governance and policies
^^^^^^^^^^^^^^^^^^^^^^^

- `Code of conduct <https://ubuntu.com/community/code-of-conduct>`_

Get involved
^^^^^^^^^^^^

- `Get support <https://discourse.charmhub.io/>`_
- `Join our online chat <https://matrix.to/#/#charmhub-charmdev:ubuntu.com>`_
- :ref:`Contribute <how_to_contribute>`

Releases
^^^^^^^^

- :ref:`Release notes <release_notes_index>`

Thinking about using the bingo Operator for your next project? 
`Get in touch <https://matrix.to/#/#charmhub-charmdev:ubuntu.com>`_!

.. vale Canonical.013-Spell-out-numbers-below-10 = NO
.. vale Canonical.500-Repeated-words = NO

.. toctree::
    :hidden:
    :maxdepth: 1

    Tutorial <tutorial/index>
    How-to guides <how-to/index>
    Reference <reference/index>
    Explanation <explanation/index>
    Release notes <release-notes/index>
