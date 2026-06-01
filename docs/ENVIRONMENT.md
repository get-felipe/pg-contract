# Environment

## Stack

- Language: Go 1.26.
- Postgres driver: `github.com/jackc/pgx/v5`.
- Runtime target: local CLI.
- Database target: Postgres.

## Local Setup

```sh
git clone https://github.com/get-felipe/pg-contract.git
cd pg-contract
go version
go test ./...
go build -o bin/pg-contract ./cmd/pg-contract
```

Run the skeleton:

```sh
go run ./cmd/pg-contract --help
go run ./cmd/pg-contract version
```

Generate an initial config:

```sh
go run ./cmd/pg-contract init \
  --queries examples/basic/queries \
  --out -
```

Run the current checker:

```sh
go run ./cmd/pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --schema-before examples/basic/schema-before.sql \
  --schema-after examples/basic/schema-after.sql \
  --queries examples/basic/queries
```

Use explicit parameter types from config:

```sh
go run ./cmd/pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --queries examples/typed-params/queries \
  --config examples/typed-params/pg-contract.yaml
```

When `pg-contract.yaml` exists in the current directory, `check` loads it automatically. Use `--no-config` to skip autodetection.

Use JSON for machine-readable output:

```sh
go run ./cmd/pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --queries examples/basic/queries \
  --format json
```

Use GitHub workflow command annotations:

```sh
go run ./cmd/pg-contract check \
  --before-url "$PG_CONTRACT_BEFORE_URL" \
  --after-url "$PG_CONTRACT_AFTER_URL" \
  --queries examples/basic/queries \
  --format github
```

Use disposable databases when supplying schema files.

The Makefile loads `.env.local` automatically when it exists. Start from:

```sh
cp .env.example .env.local
```

Then point the URLs at two scratch databases.

## Make Targets

```sh
make fmt
make test
make build
make check
make example-init
make example-basic
make example-missing-table
make example-ambiguous-column
make example-typed-params
make example-basic FORMAT=json
make example-typed-params FORMAT=json
make example-basic FORMAT=github
make test-integration
make tidy
```

## GitHub Actions

The initial CI workflow:

- Checks out the repository.
- Installs Go using `actions/setup-go`.
- Checks `gofmt`.
- Runs `go test ./...`.
- Builds `bin/pg-contract` from `./cmd/pg-contract`.

## Integration Tests

Unit tests run without Postgres:

```sh
go test ./...
```

The Postgres integration test is skipped unless both variables are set:

```sh
export PG_CONTRACT_TEST_BEFORE_URL="postgres://..."
export PG_CONTRACT_TEST_AFTER_URL="postgres://..."
make test-integration
```

The two URLs must point to separate scratch databases. The test creates and drops uniquely named tables, but it still executes schema SQL against those databases.
