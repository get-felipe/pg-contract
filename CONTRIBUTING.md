# Contributing

Thanks for considering a contribution.

`pg-contract` is intentionally narrow: it validates whether existing Postgres application SQL still prepares against a proposed schema. Contributions should strengthen that core contract before expanding the product surface.

## Good First Contributions

- Add a small SQL fixture for a Postgres breakage the tool should explain better.
- Improve an error reason or suggested fix.
- Add tests around query loading, config validation, or reporters.
- Clarify docs, examples, or GitHub Actions usage.
- Report a reproducible false positive or false negative.

## Before Opening an Issue

Please include:

- `pg-contract` version or commit.
- Postgres version.
- Minimal before schema, after schema, and query file.
- The command you ran.
- Expected result and actual output.

Do not include real credentials, production connection strings, customer data, or proprietary schema details.

## Local Setup

```sh
git clone https://github.com/get-felipe/pg-contract.git
cd pg-contract
make check
```

For integration tests and examples, create two disposable Postgres databases and copy `.env.example`:

```sh
createdb pg_contract_before
createdb pg_contract_after
cp .env.example .env.local
make test-integration
make example-basic
```

Only use disposable databases for examples and integration tests. Schema files passed to `--schema-before` and `--schema-after` are executed against the supplied URLs.

## Development Commands

```sh
make fmt
make test
make build
make check
make test-integration
make release-check
```

## Pull Request Guidelines

- Keep changes small and scoped.
- Add or update tests for behavior changes.
- Update README or docs when commands, output, config, or scope changes.
- Preserve exit code semantics: `0` clean, `1` breaking query contract, `2` invalid input/runtime/invalid-before.
- Do not print database URLs, passwords, raw query text, or runtime parameter values in reports.
- Avoid new dependencies unless they remove meaningful complexity.
- Prefer compatibility fixtures over broad abstractions when adding diagnostic coverage.

## Design Principles

- Postgres is the source of truth for query validity.
- Valid-before/fail-after is the core breaking-change signal.
- Reports should explain why a query broke and where to fix it.
- The CLI should integrate with existing migration workflows, not own them.
- False positives and false negatives are product bugs.

## Project Boundaries

In scope:

- Postgres.
- Local CLI and GitHub Action.
- SQL files and manifest-based query contracts.
- Explicit Postgres parameter types through `pg-contract.yaml`.
- Diagnostics grounded in Postgres errors.

Out of scope for now:

- Migration generation.
- Production migration execution.
- Lock and downtime analysis.
- Multi-database support.
- ORM-specific extractors.
- SaaS, hosted dashboards, or cloud-required workflows.

## License of Contributions

By contributing, you agree that your contribution is licensed under the repository's MIT license.
