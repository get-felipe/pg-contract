# Query Manifest v0.2

Status: implemented in alpha

This document records the `pg-contract.yaml` v0.2 manifest shape and the design constraints behind it.

## Goals

- Keep the current alpha config working.
- Support larger repositories with more than one query root.
- Make schema and `search_path` assumptions explicit when they affect query preparation.
- Keep SQL query names as the stable contract identity.
- Avoid becoming a build system, ORM config, or migration runner.

## References Considered

- `sqlc` config v2 uses a top-level `version` and a list of SQL units with `schema`, `queries`, and `engine` fields. Its `schema` and `queries` values can be single paths or lists of paths.
- Atlas project config separates environment-level settings and supports multiple schema sources, including ordered composition of schema inputs.
- `stripe/pg-schema-diff` keeps the schema source plain SQL and validates plans against Postgres, which matches the direction of keeping `pg-contract` close to real Postgres behavior.

## Proposed Shape

```yaml
version: "0.2"

defaults:
  prepare:
    search_path:
      - public

query_sets:
  - name: app
    queries:
      - internal/db/queries
      - services/billing/queries
    schema:
      before:
        - schema/base.sql
      after:
        - schema/proposed.sql
    tags:
      - app
      - ci

  - name: reporting
    queries:
      - analytics/queries
    prepare:
      search_path:
        - reporting
        - public
    tags:
      - reporting

queries:
  customers.find:
    params:
      - uuid
    tags:
      - customer-facing

  search.count_tags:
    params:
      - text[]
```

## Field Semantics

| Field | Required | Description |
| --- | --- | --- |
| `version` | Yes for v0.2 | Manifest format version. Missing `version` means the legacy alpha shape. |
| `defaults.prepare.search_path` | No | Search path to apply before preparing queries unless a query set overrides it. |
| `query_sets` | Yes for v0.2 | Ordered list of query groups to validate. |
| `query_sets[].name` | Yes | Stable query set identifier for output and selection. |
| `query_sets[].queries` | Yes | One or more SQL files or directories. Directories are scanned recursively for `.sql` files. Accepts a string or list of strings. |
| `query_sets[].schema.before` | No | Optional SQL files applied to the before database for this set. Accepts a string or list of strings. |
| `query_sets[].schema.after` | No | Optional SQL files applied to the after database for this set. Accepts a string or list of strings. |
| `query_sets[].prepare.search_path` | No | Per-set search path override. |
| `query_sets[].tags` | No | Labels for filtering, reporting, or ownership. Tags do not affect SQL semantics. |
| `queries` | No | Per-query overrides keyed by sqlc-style query name. |
| `queries.<name>.params` | No | Ordered Postgres parameter types for `$1`, `$2`, and so on. |
| `queries.<name>.tags` | No | Per-query labels merged with query set tags. |

Schema files are applied in query set order to the supplied before and after databases. Those databases should be disposable for the check.

## Query Identity

The canonical query identity remains the sqlc-style query name:

```sql
-- name: customers.find :one
select id, email
from customers
where id = $1;
```

Per-query config is keyed by this name, not by file path. This keeps the config stable when files move. If two loaded query files declare the same name, v0.2 should fail fast with a duplicate query name error before connecting to Postgres.

Unnamed queries may still fall back to file-path-based names, but large repositories should prefer explicit `-- name:` comments.

## CLI Boundary

Keep these values as CLI, GitHub Action, or environment inputs:

- `before-url`
- `after-url`
- output format
- timeout
- CI annotation mode

Allow these values in the manifest:

- query roots
- optional schema SQL files
- preparation assumptions such as `search_path`
- parameter type overrides
- tags and metadata

This keeps secrets and runtime-specific connection details out of the repository while allowing durable query contract metadata to live beside source code.

## Backward Compatibility

The current alpha config remains valid:

```yaml
queries:
  search.count_tags:
    params:
      - text[]
```

Migration to v0.2 is mechanical:

```yaml
version: "0.2"

query_sets:
  - name: default
    queries:
      - ./queries

queries:
  search.count_tags:
    params:
      - text[]
```

`pg-contract init` still generates the alpha-compatible shape. A later release can add an explicit opt-in such as `pg-contract init --manifest-version 0.2`.

## Output Stability

Existing JSON and GitHub output fields should remain stable:

- query name
- file and line
- status
- SQLSTATE
- reason
- suggestion
- result shape metadata when returned columns change

v0.2 adds optional `query_set` and `tags` fields for manifest results, but existing fields are not renamed or removed.

## Rejected Alternatives

### Keep Only A Name-Keyed `queries` Map

This is simple for small projects, but it does not describe multiple query directories, per-set schema assumptions, or ownership metadata.

### Store Database URLs In The Manifest

Rejected because connection strings are runtime inputs and may contain secrets. They belong in CLI flags, environment variables, or CI secrets.

### Copy The Full `sqlc` Config Shape

`sqlc` is a code generator and needs generator-specific settings. `pg-contract` should borrow the idea of versioned SQL units, not the whole generator config model.

### Add Environment Blocks Now

Environment blocks are useful in mature tools, but v0.2 should first solve query grouping and preparation assumptions. Runtime before/after databases should stay explicit at the CLI boundary.

### One Manifest File Per Query Set

Multiple files would reduce merge conflicts in large repositories, but they make discovery, validation, and CI usage less obvious. A single manifest with multiple `query_sets` is easier to reason about for v0.2.

## Implementation Decisions

- `--queries`, `--schema-before`, and `--schema-after` are mutually exclusive with v0.2 `query_sets`. In manifest mode, query sets own query and schema inputs.
- `--query-set` is repeatable. When provided, only selected query sets are loaded and checked; selected sets still run in manifest order. Full manifest runs still validate every query override.
- `--tag` is repeatable. When provided, only queries with any selected query-set or per-query tag are checked. Tags still do not affect SQL preparation semantics.
- Manifest paths are resolved relative to the manifest file.
- Strict YAML decoding is preserved so typos fail early.
