# pg-contract

Postgres query compatibility checks for schema changes.

`pg-contract` answers one narrow question:

> Do the application SQL queries that work against the current Postgres schema still prepare successfully against the proposed schema?

It is not a schema diff tool, migration runner, ORM, or hosted service. It validates real SQL against real Postgres schemas and reports the exact query, source file, SQLSTATE, diagnostic reason, and returned-column contract when a schema change breaks an existing query.

## Status

Pre-release alpha. The core `check` command works for `.sql` query files, user-provided Postgres URLs, optional schema SQL files, query result shape comparison, query manifest v0.2, JSON output, and GitHub Actions annotations.

The project is useful for early adopters, but command flags and report fields may still change before a stable release.

## Why pg-contract?

Schema diff tools can tell you what changed in the database. Migration linters can warn about lock hazards and unsafe operations. Those are useful, but they do not answer whether your existing application queries still compile against the new schema.

`pg-contract` is designed for that missing contract:

- Prepare the same SQL against a before schema and an after schema.
- Treat valid-before/fail-after as a breaking application contract change.
- Treat changed result column names, order, or types as a breaking application contract change.
- Use Postgres itself as the source of truth for query validity.
- Keep diagnostics close to the query file that needs attention.
- Fit into local development and pull-request checks without owning your migration workflow.

## Install

From a tagged release:

```sh
go install github.com/get-felipe/pg-contract/cmd/pg-contract@v0.1.0-alpha.7
```

From source:

```sh
git clone https://github.com/get-felipe/pg-contract.git
cd pg-contract
make build
./bin/pg-contract version
```

GitHub Release archives are produced for Linux, macOS, and Windows on `amd64` and `arm64`. Each release includes a SHA-256 checksum file.

See [Installation](docs/INSTALLATION.md) for release archive downloads, checksum verification, and platform-specific commands.

## Quick Start

Create two disposable Postgres databases: one for the current schema and one for the proposed schema.

```sh
createdb pg_contract_before
createdb pg_contract_after
```

Run a compatibility check:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --schema-before examples/basic/schema-before.sql \
  --schema-after examples/basic/schema-after.sql \
  --queries examples/basic/queries
```

Example output:

```text
FAIL customers.find_customer
File: examples/basic/queries/find_customer.sql:2

Reason:
  A column referenced by this query does not exist in the target schema.

Postgres error:
  ERROR: column "email" does not exist
  SQLSTATE: 42703

Impact:
  This query worked before the schema change and fails after it.

Suggested fix:
  Keep the old column until deployed application code no longer reads it, or update this query before removing the column.
```

Exit codes:

- `0`: no valid-before/fail-after breakages were found.
- `1`: at least one existing query breaks against the after schema.
- `2`: input, runtime, or invalid-before error.

## Query Files

`pg-contract` scans `.sql` files recursively. Use sqlc-style names when possible:

```sql
-- name: customers.find_customer :one
select id, email
from customers
where id = $1;
```

If no `-- name:` comment exists, the file path is used as the query name.

## Configuration

Some SQL parameters cannot be inferred by Postgres during `PREPARE`. Generate a starter config when a query needs explicit parameter types:

```sh
pg-contract init --queries ./queries --out pg-contract.yaml
```

```yaml
queries:
  search.count_tags:
    params:
      - text[]
```

`check` automatically loads `pg-contract.yaml` from the current directory when it exists. Use `--config path/to/file.yaml` for an explicit path, or `--no-config` to disable config loading.

For larger repositories, use manifest v0.2 to put query roots and schema files in config:

```yaml
version: "0.2"

query_sets:
  - name: app
    queries:
      - ./queries
    schema:
      before:
        - ./schema-before.sql
      after:
        - ./schema-after.sql
```

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --config pg-contract.yaml
```

Run one or more manifest query sets when a PR only touches part of a larger repository:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --config pg-contract.yaml \
  --query-set app
```

Use tags for narrower checks inside selected query sets:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --config pg-contract.yaml \
  --query-set app \
  --tag customer-facing
```

See [Configuration](docs/CONFIGURATION.md) for details.

## Output Formats

Text output is intended for humans:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --queries ./queries
```

JSON output is intended for tools:

```sh
pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --queries ./queries \
  --format json
```

JSON reports include status, summary counts, query source locations, SQLSTATE, diagnostic fields, and result shape changes. They do not include database URLs or raw query text.

See [Diagnostics](docs/DIAGNOSTICS.md) for the SQLSTATE mappings behind text, JSON, and GitHub annotation output.

## GitHub Actions

```yaml
- uses: get-felipe/pg-contract@v0.1.0-alpha.7
  with:
    before-url: ${{ secrets.PG_CONTRACT_BEFORE_URL }}
    after-url: ${{ secrets.PG_CONTRACT_AFTER_URL }}
    queries: ./queries
```

The action emits GitHub workflow command annotations with file and line metadata. Use a pinned release tag or commit SHA in production workflows.

See [GitHub Actions](docs/GITHUB_ACTIONS.md) for a complete workflow.

## Known Limitations

- Only Postgres is supported.
- Queries must be available as `.sql` files; SQL extraction from application code is not implemented.
- Dynamic SQL and runtime string interpolation are not analyzed.
- Queries are prepared, not executed; data-dependent runtime failures are out of scope.
- The tool does not create production-safe migrations or analyze lock impact.
- The user provides the before and after Postgres databases.
- Pre-release output fields and flags may change.

## Roadmap

Near-term:

- Prepare the next alpha release with broader fixture coverage.
- Improve diagnostics for more Postgres SQLSTATEs.
- Add PR-focused documentation for common CI setups.
- Document result-shape limitations and add focused fixtures for additional SQLSTATEs.
- Add tag filtering for manifest workflows.

Later:

- Optional query extraction adapters.
- Richer GitHub PR summaries.
- Package manager distribution once the release shape is stable.

## Development

Requirements:

- Go 1.26 or newer.
- Git.
- Two Postgres databases for integration checks.

Common commands:

```sh
make test
make build
make check
make test-integration
make release-check
```

Run examples locally:

```sh
cp .env.example .env.local
make example-basic
make example-missing-table
make example-ambiguous-column
make example-view-changed
make example-function-signature
make example-enum-value
make example-search-path
make example-result-shape
make example-typed-params
make example-manifest-v02
make example-basic FORMAT=json
```

Use disposable databases when passing `--schema-before` or `--schema-after`; those files are executed against the supplied URLs.

## Project Boundaries

In scope:

- Postgres query compatibility checks.
- Local CLI and GitHub Action workflows.
- SQL files and explicit query metadata.
- Clear diagnostics for valid-before/fail-after breakages.
- Returned-column shape checks for names, order, and types.

Out of scope:

- Migration generation.
- Migration execution in production.
- Lock/downtime planning.
- ORM-specific behavior.
- Hosted dashboards or cloud-required workflows.

## Documentation

- [Project brief](docs/PROJECT_BRIEF.md)
- [Installation](docs/INSTALLATION.md)
- [Changelog](CHANGELOG.md)
- [Plan](docs/PLAN.md)
- [Environment](docs/ENVIRONMENT.md)
- [Configuration](docs/CONFIGURATION.md)
- [Diagnostics](docs/DIAGNOSTICS.md)
- [Query manifest v0.2](docs/QUERY_MANIFEST_V02.md)
- [GitHub Actions](docs/GITHUB_ACTIONS.md)
- [Releasing](docs/RELEASING.md)
- [Reference analysis](docs/REFERENCE_ANALYSIS.md)

## Contributing

Contributions are welcome, especially small fixtures, diagnostic improvements, documentation fixes, and focused bug reports. Read [Contributing](CONTRIBUTING.md) before opening a pull request.

## Security

Please do not open public issues with sensitive database URLs, credentials, schema details, or vulnerability proof-of-concepts. Read [Security Policy](SECURITY.md) for reporting guidance.

## License

MIT. See [LICENSE](LICENSE).
