# Releasing the Go modules

Go has no package registry to upload to: `go get` downloads modules from their git repository
through the public module proxy (proxy.golang.org), and a version is a git tag. This repository
holds two modules, released together with the same version:

| Module | Tag | Contents |
| --- | --- | --- |
| `github.com/EdiFabric/edifabric-go-bindings` | `v1.1.1` | the bindings |
| `github.com/EdiFabric/edifabric-go-bindings/native` | `native/v1.1.1` | the four native libraries in `native/bin/<rid>/`, embedded with `go:embed` |

The `native` package embeds only the library for the target (`windows/amd64`, `linux/amd64`,
`linux/arm64`, `darwin/arm64`) and passes it to `edifabricx12.RegisterBundledLibrary`, which
writes it to the user cache folder when no other library is found. Because the proxy serves
whatever the tag contains, the libraries are committed to git in the release commit. The
libraries come from the public downloads at `https://www.edifabric.com/download/`, so release only
after the native build for that version is uploaded there.

| File | Purpose |
| --- | --- |
| `bundled.go` | `RegisterBundledLibrary` and the extraction to the cache folder |
| `native/` | the `native` module; its `go.mod` requires the bindings at the release version |
| `scripts/fetch-natives.ps1`, `.sh` | download the four libraries into `natives/<rid>/` |
| `scripts/stage-natives.sh` | copy them into `native/bin/<rid>/` (`release.ps1` does this itself) |
| `scripts/smoke-test/` | a consumer program that checks the embedded library loads |
| `scripts/release.ps1`, `.sh` | stage, smoke-test, commit, tag, push, and warm the proxy |
| `.github/workflows/release-go.yml` | the same pipeline in GitHub Actions, with a smoke test on every platform |

## One-time setup

1. The repository must be public on GitHub as `github.com/EdiFabric/edifabric-go-bindings`; the
   module path depends on it.
2. GitHub: create the environment `go-release` (**Settings > Environments**), optionally with
   required reviewers. The workflow pushes the release commit and tags with the workflow token.
   If the default branch is protected against direct pushes, allow
   `github-actions[bot]` to bypass the rule, or release from your machine instead.
3. For releasing from your machine only: Go 1.25+ and push rights to the repository.

## Release

1. Upload the new native build to `https://www.edifabric.com/download/`.
2. Update the version in `README.md` and commit everything; a real release refuses to run with
   uncommitted changes.
3. Validate:
   - GitHub: **Actions > Release Go modules > Run workflow** with the version and **dry-run**
     checked
   - or locally: `./scripts/release.ps1 -Version 1.1.1 -DryRun` (Windows) or
     `scripts/release.sh 1.1.1 --dry-run`. This leaves the libraries in `native/bin/` and the
     version in `native/go.mod` as uncommitted changes; discard them or let the release commit them.
4. Release: run the workflow with **dry-run** cleared, or `./scripts/release.ps1 -Version 1.1.1`
   or `scripts/release.sh 1.1.1`. Both commit `native/` as `Release v1.1.1`, create the tags
   `v1.1.1` and `native/v1.1.1`, push the branch and tags, and ask proxy.golang.org to fetch
   both modules. The workflow also creates the GitHub release.
5. Check: `go list -m github.com/EdiFabric/edifabric-go-bindings/native@v1.1.1` from any folder.
   The listing on pkg.go.dev appears within a few minutes.

The proxy and checksum database keep a version permanently, even if the tag is deleted, so never
move or reuse a tag; release a new version instead. To steer users away from a bad version, add a
[`retract`](https://go.dev/ref/mod#go-mod-file-retract) directive to `go.mod` in the next release.

Each release adds about 34 MB of libraries to the git history. If that becomes a problem, move
the `native` module to its own repository.
