(how_to_contribute)=

# How to contribute

We welcome contributions to the bingo charm's documentation. This documentation is built
with Sphinx and MyST Markdown and published on Read the Docs.

## Report an issue

File a bug or suggestion on [GitHub](https://github.com/canonical/bingo/issues).

## Make a change

1. Fork the repository and create a branch for your change.
2. Edit the relevant file(s) under `docs/`.
3. Build the documentation locally to check your changes:
   ```
   cd docs
   make run
   ```
4. Open a pull request. The `automatic-doc-checks.yml`, `markdown-style-checks.yml`, and
   `check-removed-urls.yml` workflows will validate your changes.

See [CONTRIBUTING.md](https://github.com/canonical/bingo/blob/main/CONTRIBUTING.md) for
code contribution guidelines.
