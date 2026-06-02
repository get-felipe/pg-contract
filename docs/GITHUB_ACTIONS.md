# GitHub Actions

`pg-contract` ships as a composite GitHub Action and can also emit workflow command annotations directly with `--format github`.

The action expects your job to prepare two Postgres databases that represent the before and after schemas. It does not create or migrate those databases by itself.

## Composite Action

Use a pinned tag or commit SHA in real workflows. The examples use the planned first alpha tag.

```yaml
name: pg-contract

on:
  pull_request:

permissions:
  contents: read

jobs:
  check:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v6

      # Prepare the before/after Postgres databases here.

      - name: Check Postgres query compatibility
        uses: get-felipe/pg-contract@v0.1.0-alpha.3
        with:
          before-url: ${{ secrets.PG_CONTRACT_BEFORE_URL }}
          after-url: ${{ secrets.PG_CONTRACT_AFTER_URL }}
          queries: ./queries
```

The action always uses GitHub annotation output. Breaking queries appear as workflow errors with file and line metadata.

## Inputs

| Input | Required | Default | Description |
| --- | --- | --- | --- |
| `before-url` | Yes | | Postgres URL for the current/base schema. |
| `after-url` | Yes | | Postgres URL for the proposed/target schema. |
| `queries` | Yes | | Directory containing `.sql` query files. |
| `schema-before` | No | | Optional SQL file to load into `before-url` before checking. |
| `schema-after` | No | | Optional SQL file to load into `after-url` before checking. |
| `config` | No | | Optional `pg-contract.yaml` path. |
| `no-config` | No | `false` | Set to `true` to disable automatic config loading. |
| `timeout` | No | `30s` | Per-connection timeout. |

The command auto-loads `pg-contract.yaml` from the workflow working directory. Set `config` if the file lives elsewhere, or set `no-config: true` if the workflow should ignore it.

Generate a starter config locally with:

```sh
pg-contract init --queries ./queries --out pg-contract.yaml
```

## CLI Step

You can also run the CLI directly from a workflow step when developing this repository or testing unreleased changes:

```yaml
steps:
  - uses: actions/checkout@v6

  - uses: actions/setup-go@v6
    with:
      go-version-file: go.mod

  - name: Check Postgres query compatibility
    env:
      PG_CONTRACT_BEFORE_URL: ${{ secrets.PG_CONTRACT_BEFORE_URL }}
      PG_CONTRACT_AFTER_URL: ${{ secrets.PG_CONTRACT_AFTER_URL }}
    run: |
      go build -o ./bin/pg-contract ./cmd/pg-contract
      ./bin/pg-contract check \
        --before-url "$PG_CONTRACT_BEFORE_URL" \
        --after-url "$PG_CONTRACT_AFTER_URL" \
        --queries ./queries \
        --format github
```

The command exits with:

- `0` when no valid-before/fail-after breakages are found.
- `1` when at least one breaking schema change is found.
- `2` when the check cannot run cleanly or a query is already invalid against the before schema.

Annotations intentionally include query names, files, lines, SQLSTATE, and concise Postgres diagnostics. They do not include database URLs or raw query text.

## Repository Self-Test

This repository includes `.github/workflows/action-self-test.yml` to exercise the composite action through `uses: ./` against a disposable PostgreSQL service container. The self-test intentionally runs one breaking example with `continue-on-error: true`, asserts that it fails, and then runs one compatible typed-parameter example that must pass.
