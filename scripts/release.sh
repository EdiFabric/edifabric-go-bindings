#!/usr/bin/env bash
# Commit the four libraries into the native module, smoke-test it, and tag and push both modules.
#
# Go modules are served from git tags, so a release is a commit with the libraries in native/bin/
# plus the tags v<version> (the bindings) and native/v<version> (the libraries). Needs Go 1.25+
# and push rights to the remote.
#
#   scripts/release.sh 1.1.1 --dry-run   stage and smoke-test without committing, tagging, or pushing
#   scripts/release.sh 1.1.1             release
set -euo pipefail
cd "$(dirname "$0")/.."
module=github.com/EdiFabric/edifabric-go-bindings

version="" dry_run=false skip_fetch=false remote=${REMOTE:-origin}
for arg in "$@"; do
    case "$arg" in
        --dry-run) dry_run=true ;;
        --skip-fetch) skip_fetch=true ;;
        -*) echo "unknown option: $arg" >&2; exit 2 ;;
        *) version=$arg ;;
    esac
done
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || { echo "usage: $0 <x.y.z> [--dry-run] [--skip-fetch]" >&2; exit 2; }
tags=("v$version" "native/v$version")

if ! $dry_run; then
    [ -z "$(git status --porcelain)" ] || { echo "Commit or stash your changes first; the release tags HEAD." >&2; exit 1; }
    for tag in "${tags[@]}"; do
        [ -z "$(git tag --list "$tag")" ] || { echo "Tag $tag already exists." >&2; exit 1; }
    done
fi

$skip_fetch || bash scripts/fetch-natives.sh natives
bash scripts/stage-natives.sh

(cd native && go mod edit -require="$module@v$version" && go mod tidy)
(cd scripts/smoke-test && go mod edit -require="$module@v$version" -require="$module/native@v$version" && go mod tidy)

go vet ./...
for target in windows/amd64 linux/amd64 linux/arm64 darwin/arm64 linux/riscv64; do
    (cd native && GOOS=${target%/*} GOARCH=${target#*/} go vet ./...)
done

smoke=$(mktemp -d)
trap 'rm -rf "$smoke"' EXIT
go build -C scripts/smoke-test -o "$smoke/smoke-test" .
(cd "$smoke" && ./smoke-test)

if $dry_run; then
    echo "Dry run: staged the libraries in native/bin and smoke-tested; nothing committed, tagged, or pushed."
    exit 0
fi

git add native scripts/smoke-test
git diff --cached --quiet || git commit -m "Release v$version"
for tag in "${tags[@]}"; do git tag -a "$tag" -m "Release $tag"; done
git push "$remote" HEAD "${tags[@]}"

# Ask the module proxy to fetch both versions so `go get` sees them right away.
(cd "${TMPDIR:-/tmp}" && GOPROXY=https://proxy.golang.org go mod download -json "$module@v$version" "$module/native@v$version")
echo "Released $module v$version and $module/native v$version."
