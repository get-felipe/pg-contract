# Plan

Build a narrow local CLI that proves the core compatibility check before adding extractors, GitHub comments, or broader ecosystem integrations. The first useful release should validate `.sql` files against two Postgres schemas and explain valid-before/fail-after breakages.

## Scope

- In: Postgres, local CLI, schema SQL files, query SQL files, manifest-based metadata, diagnostics, JSON/text output.
- Out: migration generation, production migration execution, hosted service, ORM/language extractors, multi-database support.

## Action Items

- [x] Initialize the repository with Go module, basic CLI skeleton, tests, Makefile, CI, and public repo hygiene files.
- [x] Write project brief, initial plan, reference analysis, and local maintenance notes.
- [x] Define the v0.1 query file format, including sqlc-style `-- name:` comments.
- [x] Add idempotent example fixtures for column removal, ambiguous column, missing table, changed views, function signature changes, enum value changes, and `search_path` changes.
- [x] Implement schema loading into user-provided Postgres databases.
- [x] Implement query preparation against the before and after schemas.
- [x] Compare prepared statement result column names, order, and types.
- [x] Add strict YAML config for explicit query parameter types.
- [x] Add `init` command to generate starter config from query files.
- [x] Auto-load `pg-contract.yaml` from the current directory, with `--no-config` opt-out.
- [x] Design the query manifest v0.2 shape before expanding config and CLI behavior.
- [x] Implement query manifest v0.2 parsing and execution for query sets, multiple query roots, per-set schema files, `search_path`, and tags.
- [x] Add focused manifest execution with repeatable `--query-set` selection.
- [x] Add focused manifest execution with repeatable `--tag` selection.
- [x] Classify high-value Postgres errors into clear reasons and suggested fixes.
- [x] Add JSON reporter.
- [x] Add text reporter with Postgres SQLSTATE, message, position, and structured object fields when available.
- [x] Add GitHub Actions annotation reporter.
- [x] Add env-gated integration tests backed by user-provided Postgres URLs.
- [x] Draft a GitHub Action wrapper after the CLI is useful locally.
- [x] Add a workflow self-test that exercises the composite action against disposable Postgres services.
- [x] Add a release workflow for tagged binaries and checksums.
- [x] Add changelog and issue templates for the first alpha.

## v0.1 Command Shape

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --schema-before schema-main.sql \
  --schema-after schema-pr.sql \
  --queries queries/ \
  --format text
```

Generate starter config:

```sh
pg-contract init --queries queries/ --out pg-contract.yaml
```

`check` auto-loads `pg-contract.yaml` from the current directory. Use `--config path` for an explicit file or `--no-config` to disable config loading.

Machine-readable output:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --queries queries/ \
  --format json
```

GitHub Actions annotations:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --queries queries/ \
  --format github
```

Optional manifest for parameter types:

```yaml
queries:
  customers.find_customer:
    params:
      - uuid
```

## Open Questions

- Resolved: v0.1 accepts user-provided Postgres URLs. Docker is only for project tests/examples later, not the primary product mode.
- Resolved: query files follow sqlc-style `-- name:` comments, with file-path fallback.
- Resolved: v0.1 uses `github.com/jackc/pgx/v5`.

## Risk Notes

- Dynamic SQL extraction is deliberately out of scope until the core check works.
- False positives should be treated as product bugs; the tool must make it easy to reproduce every reported failure.
- The CLI should not print full DSNs, passwords, or runtime parameter values.
