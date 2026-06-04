# Security Policy

## Supported Versions

`pg-contract` is pre-release software.

| Version | Supported |
| --- | --- |
| Latest tagged alpha | Yes |
| `main` | Best effort |
| Older pre-release tags | No |

Use the latest tagged alpha for reproducible security reports. Reports against `main` are accepted on a best-effort basis when they include the exact commit SHA.

## Reporting a Vulnerability

Please report security issues privately before opening a public issue.

Preferred path:

1. Use GitHub private vulnerability reporting if it is enabled for this repository.
2. If no private channel is available, open a minimal public issue asking for a private contact path.
3. Do not include exploit details, credentials, production schemas, database URLs, logs with secrets, or customer data in a public issue.

Include when possible:

- Affected version or commit.
- Operating system and Postgres version.
- Minimal reproduction steps.
- Expected impact.
- Whether credentials, database URLs, query text, or schema details can be exposed.
- Suggested mitigation, if known.

## What Counts as Security-Sensitive

Examples:

- Leaking database passwords or full connection strings.
- Printing raw query text or runtime parameter values when a report format should not expose them.
- Writing sensitive schema or connection details into generated artifacts.
- Unsafe GitHub Actions behavior that could expose secrets.

Correctness bugs, false positives, missing diagnostics, or unsupported SQL features are usually regular issues unless they expose sensitive data or create a security boundary bypass.

## Project Security Expectations

- Reports must not include database URLs or credentials.
- Test fixtures must use synthetic schemas and data only.
- CI examples should use disposable databases.
- GitHub Actions should use the minimum permissions required for each workflow.
- Dependencies should be kept current through Dependabot and reviewed before release.

## Disclosure

The maintainer will acknowledge valid vulnerability reports as soon as practical, investigate scope, and coordinate a fix or mitigation before public disclosure.
