# Configuration

`pg-contract.yaml` is optional. It supports two shapes:

- the legacy alpha shape for explicit query parameter types while `--queries` still comes from the CLI;
- manifest v0.2 for larger repositories that want query roots, schema SQL files, and preparation assumptions in config.

## Generate

Start from the queries already in your repository:

```sh
pg-contract init --queries ./queries --out pg-contract.yaml
```

This creates a valid no-op config:

```yaml
queries:
  search.count_tags:
    params: []
```

Fill `params` only for queries where Postgres cannot infer parameter types.

## Legacy Example

```yaml
queries:
  search.count_tags:
    params:
      - text[]
```

Then run:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --queries ./queries
```

## Discovery

`check` looks for `pg-contract.yaml` in the current directory and loads it automatically when present.

Use an explicit path when the config lives elsewhere:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --queries ./queries \
  --config path/to/pg-contract.yaml
```

Disable config loading:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --queries ./queries \
  --no-config
```

`--config` and `--no-config` cannot be used together.

## Legacy Schema

| Field | Required | Description |
| --- | --- | --- |
| `queries` | No | Map of query name to query config. |
| `queries.<name>.params` | No | Ordered Postgres parameter types for `$1`, `$2`, and so on. Empty or omitted params are treated as no-op. |
| `queries.<name>.tags` | No | Query labels used in manifest metadata. They do not affect SQL semantics. |

Query names must match the names loaded from SQL files:

```sql
-- name: search.count_tags
select array_length($1, 1) as tag_count;
```

## Type Names

The first version accepts conservative Postgres type names:

- built-in names such as `uuid`, `text`, `int8`, `jsonb`;
- schema-qualified names such as `public.order_status`;
- arrays such as `text[]`;
- multi-word names such as `timestamp with time zone`.

Unsupported type syntax is rejected before connecting to Postgres. This keeps config errors explicit and avoids embedding arbitrary SQL in generated `PREPARE` statements.

## Manifest v0.2

Add `version: "0.2"` when the config should own query discovery:

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

queries:
  customers.find:
    params:
      - uuid
    tags:
      - customer-facing
```

Then run without `--queries` or `--schema-*`:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --config pg-contract.yaml
```

Manifest paths are resolved relative to the manifest file. For example, if `--config config/pg-contract.yaml` contains `queries: queries`, `pg-contract` loads `config/queries`.

In v0.2 mode, `query_sets` define query and schema inputs. Do not mix v0.2 manifests with CLI `--queries`, `--schema-before`, or `--schema-after`.

Schema files are applied in query set order to the supplied before and after databases. Use disposable databases for manifest checks, just as with `--schema-before` and `--schema-after`.

## Manifest v0.2 Schema

| Field | Required | Description |
| --- | --- | --- |
| `version` | Yes | Must be `"0.2"` for manifest mode. Missing `version` keeps the legacy alpha shape. |
| `defaults.prepare.search_path` | No | Search path to apply before preparing queries unless a query set overrides it. |
| `query_sets` | Yes | Ordered list of query groups to validate. |
| `query_sets[].name` | Yes | Stable query set identifier. |
| `query_sets[].queries` | Yes | One or more SQL files or directories. Directories are scanned recursively for `.sql` files. |
| `query_sets[].schema.before` | No | Optional SQL files applied to the before database before preparing this set. |
| `query_sets[].schema.after` | No | Optional SQL files applied to the after database before preparing this set. |
| `query_sets[].prepare.search_path` | No | Per-set search path override. |
| `query_sets[].tags` | No | Labels merged into each result from the set. Tags do not affect SQL semantics. |
| `queries` | No | Per-query overrides keyed by sqlc-style query name. |
| `queries.<name>.params` | No | Ordered Postgres parameter types for `$1`, `$2`, and so on. |
| `queries.<name>.tags` | No | Labels merged into results for this query. |

`query_sets[].queries`, `query_sets[].schema.before`, and `query_sets[].schema.after` accept either a single string or a list of strings.

If two loaded query files use the same query name, manifest mode fails before connecting to Postgres. Prefer explicit sqlc-style names for stable identities:

```sql
-- name: customers.find :one
select id, email
from customers
where id = $1;
```

JSON output may include optional `query_set` and `tags` fields for manifest results. Existing query, status, summary, SQLSTATE, reason, and suggestion fields remain stable.

See [Query Manifest v0.2](QUERY_MANIFEST_V02.md) for design rationale, rejected alternatives, and migration notes.
