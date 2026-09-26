# Versioning

[中文](versioning.zh-CN.md)

dbx has two version numbers. They move for different reasons.

| | Where | Meaning |
|---|---|---|
| Module version | Git tag `vX.Y.Z` | What `go get github.com/fluxsce/dbx@vX.Y.Z` selects |
| Language version | `go 1.25.0` in `go.mod` | The minimum Go toolchain that can compile this module |

There is no version constant in the source. `go` resolves the tag.

## Module version

| Bump | When |
|---|---|
| `1.0.x` | Fixes in binding, scanning, or engine SQL. Signatures and the [contract](compatibility.en.md) stay. Generated SQL text and batch sizes may change. |
| `1.x.0` | New methods, optional dialect interfaces, or drivers. Existing callers still compile. |
| `2.0.0` | A break of the 1.x contract. The import path becomes `github.com/fluxsce/dbx/v2`. |

Install the newest tag:

```bash
go get github.com/fluxsce/dbx@latest
```

Pin a tag when a build must not move:

```bash
go get github.com/fluxsce/dbx@v1.0.1
```

## Language version

`go 1.25.0` means consumers compile dbx with Go 1.25 or newer. Raising that line drops every older toolchain. The line moves when the source uses a language or standard-library feature that older toolchains lack. A module tag can ship without changing it.

## Cutting a release

Versions are headings in [CHANGELOG.md](../CHANGELOG.md), in [Keep a Changelog](https://keepachangelog.com/) form.

Pushing `main` runs tests, then `.github/workflows/release.yml` does this:

1. Read the first heading that matches `## [x.y.z]`. `## [Unreleased]` does not match.
2. If `vX.Y.Z` already exists, stop.
3. Otherwise tag that commit `vX.Y.Z`, push the tag, and open a GitHub Release whose notes are the section under that heading.

To publish `v1.0.2`, insert `## [1.0.2] - YYYY-MM-DD` directly under `## [Unreleased]`, above every older version, describe the change, and push `main`. A new heading appended below the current version is ignored, because the workflow always reads the first numeric heading.

The tag points at the commit that introduced the heading. `go get` fetches that tag from the public repository. The module proxy caches it on the first request.
