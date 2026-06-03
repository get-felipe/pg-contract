# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project uses semantic version tags.

## [Unreleased]

No unreleased changes yet.

## [0.1.0-alpha.6] - 2026-06-03

### Added

- Result shape compatibility checks for returned column names, order, and types when queries prepare successfully on both schemas.

## [0.1.0-alpha.5] - 2026-06-02

### Added

- Focused manifest checks with repeatable `--query-set` selection in the CLI and `query-set` input in the GitHub Action.

## [0.1.0-alpha.4] - 2026-06-02

### Added

- Query manifest v0.2 support for `query_sets`, multiple query roots, per-set schema files, `search_path`, tags, and optional JSON manifest metadata.

## [0.1.0-alpha.3] - 2026-06-02

### Fixed

- `pg-contract version` now reports the tagged module version when installed with `go install`.

## [0.1.0-alpha.2] - 2026-06-01

### Added

- Installation documentation for pinned Go installs and GitHub Release archives with checksum verification.
- Compatibility fixtures for changed views, function signature changes, enum value changes, and `search_path` changes.
- Expanded SQLSTATE diagnostic coverage and documented ambiguous diagnostic cases.
- Query manifest v0.2 design covering query sets, multiple query roots, schema assumptions, and migration notes.

## [0.1.0-alpha.1] - 2026-06-01

### Added

- Initial `pg-contract check` command for validating `.sql` query files against before and after Postgres schemas.
- Support for user-provided before and after Postgres URLs.
- Optional schema loading through `--schema-before` and `--schema-after`.
- Recursive SQL query discovery with sqlc-style `-- name:` comments and file-path fallback names.
- Optional `pg-contract.yaml` config for explicit Postgres parameter types.
- `pg-contract init` command to generate a starter config from query files.
- Human-readable text output with SQLSTATE, Postgres diagnostics, impact, and suggested fixes.
- JSON output for machine-readable CI and tooling integrations.
- GitHub Actions annotation output through `--format github`.
- Composite GitHub Action with explicit inputs for URLs, schemas, queries, config, and timeout.
- GitHub Actions self-test using disposable Postgres service containers.
- GoReleaser configuration and release workflow for Linux, macOS, and Windows archives with SHA-256 checksums.
- Example fixtures for removed columns, missing tables, ambiguous columns, and typed parameters.
- Public project documentation, contribution guidance, security policy, code of conduct, and MIT license.

### Known Limitations

- Only Postgres is supported.
- Query extraction from application source code is not implemented.
- Dynamic SQL and runtime string interpolation are not analyzed.
- Queries are prepared, not executed.
- The user must provide disposable before and after databases.
- Pre-release command flags and report fields may still change.
