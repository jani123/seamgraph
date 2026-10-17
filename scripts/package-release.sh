#!/bin/sh
# Build release artifacts without Node/npm/Python: generated assets are embedded.
set -eu
version=${1:?usage: package-release.sh vX.Y.Z owner/repository output-directory}
repository=${2:?missing owner/repository}
out=${3:?missing output directory}
printf '%s\n' "$version" | LC_ALL=C awk '/^v[0-9]+\.[0-9]+\.[0-9]+([-.][A-Za-z0-9.-]+)?$/ {ok=1} END {exit !ok}' || { echo 'invalid release version' >&2; exit 1; }
printf '%s\n' "$repository" | LC_ALL=C awk '/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/ {ok=1} END {exit !ok}' || { echo 'invalid repository' >&2; exit 1; }
mkdir -p "$out"
out=$(cd "$out" && pwd)
commit=$(git rev-parse HEAD)
date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for os in linux darwin windows; do
 for arch in amd64 arm64; do
  binary=seamgraph
  [ "$os" != windows ] || binary=seamgraph.exe
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w -X main.version=$version -X main.commit=$commit -X main.buildDate=$date" -o "$tmp/$binary" ./cmd/seamgraph
  if [ "$os" = windows ]; then
   go run ./scripts/packagezip "$tmp/$binary" "$out/seamgraph_${os}_${arch}.zip"
  else
   tar -czf "$out/seamgraph_${os}_${arch}.tar.gz" -C "$tmp" "$binary"
  fi
  rm "$tmp/$binary"
 done
done
# Release installer pins both the originating repository and release version.
sed -e "s|^DEFAULT_REPOSITORY=.*|DEFAULT_REPOSITORY='$repository'|" -e "s|__SEAMGRAPH_VERSION__|$version|g" scripts/install.sh > "$out/install.sh"
chmod +x "$out/install.sh"
(cd "$out" && sha256sum seamgraph_*.tar.gz seamgraph_*.zip install.sh > SHA256SUMS)
