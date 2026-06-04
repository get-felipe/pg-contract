# Reference Analysis

This document captures the current product-space reading that should guide early scope decisions.

Last reviewed: 2026-06-03.

## Summary

The database migration/tooling space is crowded. `pg-contract` should not compete as another migration generator, migration runner, code generator, language server, or SQL review platform. The open lane is narrower:

> standalone, local, framework-agnostic compatibility checks for existing application SQL against a proposed Postgres schema.

The closest conceptual match is `sqlc verify`. The strongest adjacent product found in the current scan is `pGenie`. Neither fully replaces the current `pg-contract` wedge because one is tied to the sqlc Cloud workflow and the other is primarily a type-safe SDK generator.

## References Reviewed

| Project | Relevant Signal | Direction for pg-contract |
| --- | --- | --- |
| [stripe/pg-schema-diff](https://github.com/stripe/pg-schema-diff) | Strong Go repo structure, CLI + library, Postgres-specific scope, temporary database validation, migration hazards. | Stay focused, make examples concrete, validate behavior against real Postgres. Do not duplicate migration generation. |
| [sqlc verify](https://docs.sqlc.dev/en/latest/howto/verify.html) | Directly validates existing queries against new schema changes and reports SQLSTATE errors. The documented workflow requires pushing schemas and queries to sqlc Cloud and selecting a previously pushed tag. | Treat this as the primary conceptual reference. Differentiate by being local, standalone, and not tied to sqlc-generated code or a hosted state store. |
| [pGenie](https://pgenie.io/docs/) | SQL-first Postgres tool that validates migrations and parameterized queries against a real Postgres instance, then generates typed client SDKs and signature files. | Watch closely. It validates the same user pain, but its product shape is code generation. Keep `pg-contract` a small gate for teams that do not want generated clients or signature artifacts. |
| [Postgres Language Server](https://pgtools.dev/latest/features/type_checking/) | Uses a real database connection to validate SQL in editor workflows and exposes migration checking commands. | Do not compete as an editor/LSP experience. Borrow the principle of real-Postgres validation while keeping CI and repository contracts first. |
| [squawk](https://github.com/sbdchd/squawk) | Lints Postgres migrations and SQL to prevent downtime and promote schema best practices. | Avoid positioning as a downtime/migration linter. Integrate later if useful. |
| [pgfence](https://pgfence.com/) | Focuses on migration risk, locks, rewrites, LSP, CLI, and GitHub Action workflows. | Confirms demand for PR-time DB safety, but reinforces that lock analysis is not the first wedge. |
| [Atlas](https://github.com/ariga/atlas) | Broad schema-as-code, migration planning/linting/apply, ORM support, drift detection, CI/CD. | Do not build a platform. Keep pg-contract composable and CLI-first. |
| [pgroll](https://github.com/xataio/pgroll) | Zero-downtime Postgres migration tool that keeps old and new schema versions available during rollout. | Separate category. `pg-contract` should remain migration-tool agnostic and useful beside expand/contract tools. |
| [Bytebase](https://github.com/bytebase/bytebase) | Full database DevSecOps platform with SQL review, RBAC, audit, GitOps, drift detection. | Avoid dashboards and governance features in the OSS core. |
| [GitHub repository best practices](https://docs.github.com/en/repositories/creating-and-managing-repositories/best-practices-for-repositories) | Public repos benefit from README, license, contribution guidelines, code of conduct, and security guidance. | Include community/maintenance files from the start, but keep them lightweight. |

## Current Positioning

Short:

> A standalone local query-contract gate for Postgres schema changes.

Expanded:

> `pg-contract` validates existing application SQL against before and after Postgres schemas, then reports exactly which query contract broke. It is not a migration planner, migration runner, code generator, LSP, or hosted schema registry.

This positioning keeps the product defensible beside stronger adjacent tools:

- Compared with `sqlc verify`, avoid cloud state and sqlc-specific workflows.
- Compared with `pGenie`, avoid generated SDK adoption and signature-file management.
- Compared with migration tools, avoid owning how schemas change; check the application contract after the proposed schema exists.

## Product Boundaries

`pg-contract` should own:

- Comparing query compatibility before and after a schema change.
- Explaining breakages in plain English with Postgres error details.
- Running locally and in CI.
- Producing text, JSON, and GitHub annotation output.
- Preserving a small manifest format for query grouping, tags, schema setup, and preparation assumptions.

`pg-contract` should not own:

- Planning migrations.
- Applying migrations.
- Estimating lock duration.
- Rewriting SQL automatically.
- Managing database environments.
- Requiring hosted schema/query snapshots.
- Generating application client code.
- Acting as a language server or editor plugin.

## Strategic Next Bets

1. Contract snapshots/baselines.
   Store the resolved query contract in a local artifact so CI can compare a proposed schema without always needing a live before database. This is the cleanest way to compete with hosted historical-state workflows while staying local.

2. Better result-shape documentation and fixtures.
   `pg-contract` already checks returned column names, order, and types. The next docs should explain what this does and does not prove.

3. PR-focused usage guides.
   Teams need copy-pasteable workflows for common setups: migration files, schema dumps, ephemeral databases, and externally managed before/after URLs.

## README Lessons

Reference repos that work well show the product in the first screen:

- One sentence that says what it does.
- Install or run command.
- Concrete example.
- Clear non-goals or boundaries when the market is crowded.

For this project, the README should keep the "why" close to the command and output. The demo is the product.

## Technical Lessons

- Use real Postgres behavior for validation where possible.
- Avoid regex-only SQL reasoning for correctness-critical diagnostics.
- Keep the CLI usable as a library boundary later, but do not expose a public Go API before the internal model stabilizes.
- Prefer fixtures that can reproduce failures over prose-only explanations.
