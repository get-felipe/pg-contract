# Project Brief

## One-Liner

`pg-contract` finds and explains application SQL breakages caused by Postgres schema changes.

## Problem

Database schema changes are often reviewed independently from the application queries that will run against the new schema. Migration tools can say whether a migration applies, and migration linters can warn about downtime risk, but many teams still miss a basic production question:

> Does the SQL that exists today still work after this schema change?

The painful cases are ordinary:

- A column is dropped or renamed while deployed code still reads it.
- A new column makes an existing `ORDER BY created_at` ambiguous.
- A function, view, enum value, or type changes in a way existing SQL cannot tolerate.
- A `NOT NULL` change makes existing inserts fail.

## Product Thesis

The best initial product is a small local CLI, not a platform:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --schema-before schema-main.sql \
  --schema-after schema-pr.sql \
  --queries queries/
```

The tool should report the high-signal classes of breakage:

```text
valid before + invalid after = breaking schema change
valid before + valid after + changed result columns = breaking query contract
```

## Positioning

`pg-contract` is for teams that keep meaningful SQL in application repositories and want local, framework-agnostic compatibility checks.

Short positioning:

> Find and explain application SQL breakages caused by Postgres schema changes.

Longer positioning:

> A local Postgres compatibility checker for SQL-first applications. It validates existing queries against old and new schemas, then explains exactly what breaks before the change reaches production.

## Non-Goals

- Do not generate migrations.
- Do not apply migrations to production.
- Do not compete with full database DevOps platforms.
- Do not start with ORM extractors.
- Do not attempt to parse every form of dynamic SQL.
- Do not require a hosted service.

## Technical Hypothesis

The first version should trust Postgres, not a homegrown SQL analyzer. PostgreSQL `PREPARE` parses, analyzes, and rewrites a statement before execution, which gives a practical way to detect many compatibility failures without touching production data.

The first implementation can:

1. Connect to a user-provided before database.
2. Connect to a user-provided after database.
3. Validate query files with Postgres prepared statements.
4. Compare prepare outcomes.
5. Compare returned-column shape for queries that prepare successfully on both schemas.
6. Report queries that fail after or return a different column contract after.

The text reporter is for humans. The JSON reporter is for machine-readable CI integrations. The GitHub reporter emits workflow command annotations for pull request feedback. Machine-readable reports should not include database URLs or raw query text by default.

When Postgres cannot infer `$1`, `$2`, and other parameter types from context, `pg-contract.yaml` can provide explicit types that are passed to `PREPARE`.

## Target Users

- Engineers reviewing Postgres schema changes in pull requests.
- Teams with raw SQL in `.sql` files, query builders, or generated query manifests.
- Projects that do not use sqlc, or cannot use sqlc Cloud.
- OSS maintainers who want a small, scriptable check in CI.

## Success Criteria

- The README demo is understandable in under five minutes.
- `pg-contract check` finds a real valid-before/fail-after breakage.
- Output includes query id, file path, Postgres error or result-shape difference, and plain-English reason.
- The core can run locally without cloud credentials.
- CI usage is a thin wrapper around the CLI.
