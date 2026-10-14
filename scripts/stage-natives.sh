#!/usr/bin/env bash
# Copy the libraries from natives/<rid>/ (see fetch-natives.sh) into native/bin/<rid>/.
set -euo pipefail
cd "$(dirname "$0")/.."

for rid in win-x64 linux-x64 linux-arm64 osx-arm64; do
    found=false
    mkdir -p "native/bin/$rid"
    for library in natives/"$rid"/edifabric-x12-tools.{dll,so,dylib}; do
        [ -f "$library" ] || continue
        cp "$library" "native/bin/$rid/"
        found=true
    done
    $found || { echo "natives/$rid has no library; run scripts/fetch-natives.sh" >&2; exit 1; }
done
