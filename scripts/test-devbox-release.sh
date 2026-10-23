#!/bin/sh
# Test unpublished release artifacts through the same fixed-hash GitHub flake.
set -eu
repository=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
assets=$(CDPATH= cd -- "${1:?usage: test-devbox-release.sh release-artifacts-directory}" && pwd)
package=$(mktemp -d)
trap 'rm -rf "$package"' EXIT
mkdir "$package/nix"
cp "$repository/flake.nix" "$repository/flake.lock" "$package/"
cp "$assets/devbox-release.json" "$package/nix/release.json"
# A fetchurl output is addressed by its name and content hash, independently of
# its download URL. Seed the exact archives so CI can test before publication.
for archive in "$assets"/seamgraph_*.tar.gz; do
  nix-prefetch-url "file://$archive" >/dev/null
done
sh "$repository/scripts/test-devbox.sh" "path:$package#seamgraph"
