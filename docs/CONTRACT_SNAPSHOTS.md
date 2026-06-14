# Contract Snapshots

Status: `pg-contract snapshot` is implemented. `check --contract` remains planned.

`pg-contract` currently compares query compatibility by preparing each query against two live Postgres schemas: a before database and an after database. This is the most direct model, but it can be awkward in CI systems where the current schema is not easy to recreate for every pull request.

Contract snapshots add a second workflow:

1. Capture the current query contract into a versioned local artifact.
2. Commit that artifact beside the manifest and queries.
3. Compare future schema proposals against that artifact without requiring a live before database.

The artifact is intentionally local. It should not require a hosted state store, code generation, or a migration framework.

## Goals

- Preserve the current two-database check flow.
- Add a local baseline flow for CI setups that can provide only the proposed schema.
- Make contract changes visible in pull request diffs.
- Keep the artifact deterministic, human-reviewable, and versioned.
- Detect query SQL drift so stale snapshots do not silently validate the wrong contract.
- Support legacy query roots and manifest v0.2 query sets/tags.

## Non-Goals

- Do not generate application client code.
- Do not infer application scanner structs.
- Do not execute queries or inspect row values.
- Do not store database URLs, credentials, raw runtime parameter values, or production data.
- Do not replace migration linters, schema diff tools, or zero-downtime migration frameworks.
- Do not require cloud state or hosted schema/query snapshots.
- Do not use "baseline" to mean a migration baseline or cumulative schema bootstrap. This baseline is only the accepted query contract.

## Reference Signals

- [`sqlc verify`](https://docs.sqlc.dev/en/latest/howto/verify.html) validates existing queries against schema changes, but the documented workflow relies on pushing schemas and queries to sqlc Cloud and verifying against a pushed tag.
- [`pGenie`](https://pgenie.io/docs/) records resolved query signatures in committed files and uses freeze files to make generated artifacts reproducible. Its product shape is code generation; `pg-contract` should only use the local-artifact lesson.
- Postgres [`PREPARE`](https://www.postgresql.org/docs/current/sql-prepare.html) remains the source of truth for snapshot capture. A snapshot records what Postgres accepted for the before contract; it does not replace Postgres validation for the proposed schema.

## CLI Shape

Generate a snapshot from a live current schema:

```sh
pg-contract snapshot \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --config pg-contract.yaml \
  --out pg-contract.lock.json
```

Compare a proposed schema against a committed snapshot:

```sh
pg-contract check \
  --contract pg-contract.lock.json \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --format text
```

Legacy query roots remain supported:

```sh
pg-contract snapshot \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --queries ./queries \
  --out pg-contract.lock.json
```

Manifest focusing should work in both commands:

```sh
pg-contract snapshot \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --config pg-contract.yaml \
  --query-set app \
  --tag customer-facing \
  --out pg-contract.lock.json

pg-contract check \
  --contract pg-contract.lock.json \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --query-set app \
  --tag customer-facing
```

## Command Semantics

### `snapshot`

`snapshot` loads queries the same way `check` loads the before side today:

- reads either `--queries` or manifest v0.2 `query_sets`;
- applies before schema files when provided by the manifest or legacy flags;
- applies configured search paths;
- resolves configured parameter types;
- prepares each selected query against the before database;
- records successful query contracts;
- fails with exit code `2` if any selected query is invalid before.

The command refuses to overwrite an existing snapshot unless `--force` is supplied. `--out -` writes deterministic JSON to stdout.

### `check --contract`

`check --contract` replaces the live before database with the snapshot:

- reads the snapshot file;
- loads the current query files/config unless `--contract-only` is introduced later;
- validates that each selected query still matches the snapshot identity;
- connects only to `--after-url`;
- prepares each selected query against the after database;
- compares the after outcome and result shape against the snapshot.

The existing two-database flow remains unchanged when `--contract` is absent.

## Snapshot Format

Initial filename: `pg-contract.lock.json`.

Top-level shape:

```json
{
  "version": "0.1",
  "tool": {
    "name": "pg-contract",
    "version": "0.1.0-alpha.7"
  },
  "source": {
    "config": "pg-contract.yaml",
    "mode": "manifest-v0.2"
  },
  "scope": {
    "complete": true,
    "query_sets": [],
    "tags": []
  },
  "queries": [
    {
      "name": "customers.find_customer",
      "query_set": "app",
      "tags": ["customer-facing"],
      "file": "examples/basic/queries/find_customer.sql",
      "line": 2,
      "sql_sha256": "sha256:...",
      "params": ["uuid"],
      "result_shape": [
        {
          "position": 1,
          "name": "id",
          "type": "uuid"
        }
      ]
    }
  ]
}
```

Field rules:

| Field | Required | Notes |
| --- | --- | --- |
| `version` | Yes | Snapshot format version, independent from report JSON version. |
| `tool.name` | Yes | Always `pg-contract`. |
| `tool.version` | No | Helpful for debugging, not part of compatibility decisions. |
| `source.config` | No | Relative path when a config file was used. |
| `source.mode` | Yes | `legacy` or `manifest-v0.2`. |
| `scope.complete` | Yes | `true` when the snapshot represents the full loaded config/query root, `false` for focused selections. |
| `scope.query_sets` | No | Selected query sets used to generate the snapshot. Empty means no query-set filter. |
| `scope.tags` | No | Selected tags used to generate the snapshot. Empty means no tag filter. |
| `queries[].name` | Yes | Stable query identity. |
| `queries[].query_set` | No | Present for manifest results. |
| `queries[].tags` | No | Merged query-set and per-query tags. |
| `queries[].file` | Yes | Repository-relative path when possible. |
| `queries[].line` | Yes | First SQL line used for annotations. |
| `queries[].sql_sha256` | Yes | Hash of normalized query SQL as loaded by `pg-contract`. |
| `queries[].params` | No | Configured parameter type names. |
| `queries[].result_shape` | Yes | Ordered returned-column contract. Initial comparison should use position, name, and formatted Postgres type. |

The first format should avoid storing raw SQL. The query hash proves whether the loaded SQL still matches the captured contract without exposing query text in generated artifacts.

`type_oid` and `type_modifier` may be useful diagnostic fields, but they should not be the primary compatibility contract in the first implementation. OIDs for user-defined types can differ across databases even when the visible type contract is equivalent.

## Comparison Rules

`check --contract` should classify results with the existing statuses where possible:

| Scenario | Proposed Status |
| --- | --- |
| Snapshot query prepares after and result shape matches | `ok` |
| Snapshot query prepares after but result shape differs | `breaking` with `shape_change` |
| Snapshot query fails after | `breaking` with Postgres diagnostics |
| Current query hash differs from snapshot | exit `2` by default, because the baseline no longer represents the query being checked |
| Query exists in snapshot but selected current query file is missing | exit `2` by default |
| Current selected query does not exist in snapshot | exit `2` by default |
| Query was invalid during snapshot generation | snapshot command fails; invalid contracts are not recorded |

Strict default behavior avoids silently accepting stale baselines. Later releases can add explicit flags such as `--allow-new-queries`, `--allow-missing-queries`, or `--update-contract`.

Focused snapshots generated with `--query-set` or `--tag` should be marked with `scope.complete: false`. This prevents users and automation from mistaking a partial contract for complete repository coverage.

## Manifest v0.2 Behavior

Snapshots should preserve manifest selection metadata:

- `query_set` and `tags` are stored for reporting and filtering.
- `--query-set` and `--tag` on `check --contract` filter snapshot entries and current query loading.
- Full manifest runs validate every query override, matching current behavior.
- Focused runs may skip validation of unselected per-query overrides, matching current `--query-set`/`--tag` behavior.

Schema files in manifest mode are used only during `snapshot` generation and during the after-side setup for `check --contract`. The snapshot is not a schema dump.

## Reporting

The text, JSON, and GitHub reporters should keep the same user-facing contract:

- show the query name, file, and line;
- show SQLSTATE diagnostics when after prepare fails;
- show result-shape differences when after prepare succeeds but contract metadata changes;
- include `query_set` and `tags` when available;
- avoid database URLs, raw SQL, credentials, and runtime parameter values.

JSON reports can add optional baseline metadata later:

```json
{
  "contract": {
    "file": "pg-contract.lock.json",
    "version": "0.1"
  }
}
```

## Implementation Plan

### Phase 1: Shared Contract Model

- Add an internal package for serializing/deserializing snapshot contracts.
- Reuse `check.ResultColumn` where possible, but make the snapshot JSON shape explicit.
- Add query SQL hashing in `internal/query` or a small shared helper.
- Add tests for deterministic encoding, decoding, and hash mismatch behavior.

Status: implemented as the internal contract model and deterministic serialization foundation.

### Phase 2: `snapshot` Command

- Add CLI command parsing for `pg-contract snapshot`.
- Refactor check loading so snapshot generation can reuse query discovery, schema application, search path setup, parameter resolution, and prepare logic.
- Write `pg-contract.lock.json`.
- Add examples and documentation.

Status: implemented for legacy query roots and manifest v0.2 query sets/tags.

### Phase 3: `check --contract`

- Add `check.Options.ContractPath`.
- Add a baseline check path that connects only to `--after-url`.
- Compare after outcomes against stored snapshot result shape.
- Extend reporters only if needed for baseline metadata.

### Phase 4: GitHub Action Input

- Add optional `contract` input to the Action.
- Document workflows that generate snapshots on main and validate PRs against the committed file.

## Open Questions

- Should `check --contract` load current query files by default, or should the first release support a pure `--contract` mode that trusts the snapshot completely?
- Should the first implementation support snapshot updating, or require regenerating the file explicitly?

## Recommended Defaults

- Store normalized SQL hash, not raw SQL.
- Omit `created_at` from the first implementation for stable diffs.
- Preserve the loaded query file path in the first implementation.
- Treat new, missing, or changed query hashes as exit code `2`.
- Require explicit snapshot regeneration for contract updates.
- Implement snapshot generation before `check --contract` so the artifact format is tested in isolation.
