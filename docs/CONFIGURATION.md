# Configuration

`pg-contract.yaml` is optional. Use it when Postgres cannot infer query parameter types during `PREPARE`.

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

## Example

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

## Schema

| Field | Required | Description |
| --- | --- | --- |
| `queries` | No | Map of query name to query config. |
| `queries.<name>.params` | No | Ordered Postgres parameter types for `$1`, `$2`, and so on. Empty or omitted params are treated as no-op. |

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
