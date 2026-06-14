# Releasing

Releases are created from Git tags with GoReleaser.

## Local Checks

Validate the release configuration:

```sh
make release-check
```

Build a local snapshot without publishing:

```sh
make release-snapshot
```

Snapshot artifacts are written to `dist/`, which is ignored by Git.

Before tagging, update `CHANGELOG.md` so the release section matches the tag and date.

## Publish

Before publishing:

- [ ] Run `make check`.
- [ ] Run `make test-integration` against disposable Postgres databases.
- [ ] Run `make release-check`.
- [ ] Run `make release-snapshot`.
- [ ] Confirm `CHANGELOG.md` has the release version and date.
- [ ] Confirm README and GitHub Action examples point at the intended tag.
- [ ] Confirm there are no credentials, local paths, or private notes in versioned files.

Create and push the semantic version tag:

```sh
git tag v0.1.0-alpha.8
git push origin v0.1.0-alpha.8
```

The release workflow runs only for tags matching `v*.*.*`. It builds Linux, macOS, and Windows binaries for `amd64` and `arm64`, uploads archives to the GitHub Release, and publishes a SHA-256 checksum file.

## Versioning

The CLI version is injected at build time from the Git tag. Local development builds continue to report `0.0.0-dev`.
