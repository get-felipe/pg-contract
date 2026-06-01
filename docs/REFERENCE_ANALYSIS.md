# Reference Analysis

This document captures the current product-space reading that should guide early scope decisions.

## Summary

The database migration/tooling space is crowded. `pg-contract` should not compete as another migration generator, migration runner, or SQL review platform. The open lane is narrower:

> local, framework-agnostic compatibility checks for existing application SQL against a proposed Postgres schema.

## References Reviewed

| Project | Relevant Signal | Direction for pg-contract |
| --- | --- | --- |
| [stripe/pg-schema-diff](https://github.com/stripe/pg-schema-diff) | Strong Go repo structure, CLI + library, Postgres-specific scope, temporary database validation, migration hazards. | Stay focused, make examples concrete, validate behavior against real Postgres. Do not duplicate migration generation. |
| [sqlc verify](https://docs.sqlc.dev/en/latest/howto/verify.html) | Directly validates existing queries against new schema changes and reports SQLSTATE errors. Requires sqlc workflow and sqlc Cloud in the documented flow. | The core concept is validated. Differentiate by being local, simple, and not tied to sqlc-generated code. |
| [squawk](https://github.com/sbdchd/squawk) | Lints Postgres migrations and SQL to prevent downtime and promote schema best practices. | Avoid positioning as a downtime/migration linter. Integrate later if useful. |
| [pgfence](https://pgfence.com/) | Focuses on migration risk, locks, rewrites, LSP, CLI, and GitHub Action workflows. | Confirms demand for PR-time DB safety, but reinforces that lock analysis is not the first wedge. |
| [Atlas](https://github.com/ariga/atlas) | Broad schema-as-code, migration planning/linting/apply, ORM support, drift detection, CI/CD. | Do not build a platform. Keep pg-contract composable and CLI-first. |
| [Bytebase](https://github.com/bytebase/bytebase) | Full database DevSecOps platform with SQL review, RBAC, audit, GitOps, drift detection. | Avoid dashboards and governance features in the OSS core. |
| [GitHub repository best practices](https://docs.github.com/en/repositories/creating-and-managing-repositories/best-practices-for-repositories) | Public repos benefit from README, license, contribution guidelines, code of conduct, and security guidance. | Include community/maintenance files from the start, but keep them lightweight. |

## Product Boundaries

`pg-contract` should own:

- Comparing query compatibility before and after a schema change.
- Explaining breakages in plain English with Postgres error details.
- Running locally and in CI.
- Producing text and JSON output.

`pg-contract` should not own:

- Planning migrations.
- Applying migrations.
- Estimating lock duration.
- Rewriting SQL automatically.
- Managing database environments.
- Hosting schema/query snapshots.

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
