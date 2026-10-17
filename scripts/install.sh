#!/bin/sh
# Release copies receive repository/version defaults from the release workflow.
set -eu
DEFAULT_REPOSITORY='jani123/seamgraph'
repository=${SEAMGRAPH_REPOSITORY:-$DEFAULT_REPOSITORY}
version=${SEAMGRAPH_VERSION:-__SEAMGRAPH_VERSION__}
bin_dir=${SEAMGRAPH_BIN_DIR:-"$HOME/.local/bin"}
languages=
usage() {
  cat <<'HELP'
Install a prebuilt SeamGraph binary (Linux/macOS, amd64/arm64).
Usage: sh install.sh [--repository owner/repo] [--version vX.Y.Z|latest]
                     [--bin-dir DIR] [--languages python,typescript,go]
The installer verifies SHA-256 checksums. Language runtimes are checked only when
--languages is supplied; existing devcontainer tools are reused. No sudo or
system package changes are performed. Git/SSH checks can be included as git,ssh.
HELP
}
fail() { printf '%s\n' "seamgraph installer: $*" >&2; exit 1; }
while [ "$#" -gt 0 ]; do
  case "$1" in
    --repository|--version|--bin-dir|--languages)
      [ "$#" -ge 2 ] || fail "missing value for $1"
      case "$1" in
        --repository) repository=$2 ;;
        --version) version=$2 ;;
        --bin-dir) bin_dir=$2 ;;
        --languages) languages=$2 ;;
      esac
      shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done
printf '%s\n' "$repository" | LC_ALL=C awk '/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/ { ok=1 } END { exit !ok }' || fail '--repository must be owner/repo'
case "$version" in __*) version=latest ;; esac
case "$version" in latest) ;; v[0-9]*) printf '%s\n' "$version" | LC_ALL=C awk '/^v[0-9]+\.[0-9]+\.[0-9]+([-.][A-Za-z0-9.-]+)?$/ {ok=1} END {exit !ok}' || fail 'invalid release version' ;; *) fail 'version must be latest or vX.Y.Z' ;; esac
case "$(uname -s)" in Linux) os=linux ;; Darwin) os=darwin ;; *) fail 'supported installer platforms: Linux and macOS; use the release assets for other platforms' ;; esac
case "$(uname -m)" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) fail 'supported architectures: amd64 and arm64' ;; esac
command -v curl >/dev/null 2>&1 || fail 'curl is required'
command -v tar >/dev/null 2>&1 || fail 'tar is required'
if command -v sha256sum >/dev/null 2>&1; then checksum=sha256sum
elif command -v shasum >/dev/null 2>&1; then checksum=shasum
else fail 'sha256sum or shasum is required'; fi
[ -n "$bin_dir" ] || fail 'installation directory is empty'
[ ! -d "$bin_dir/seamgraph" ] || fail 'destination seamgraph is a directory'
tmp=$(mktemp -d)
staged=
cleanup() { rm -rf "$tmp"; if [ -n "$staged" ]; then rm -f "$staged"; fi; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
base="https://github.com/$repository/releases"
if [ "$version" = latest ]; then base="$base/latest/download"; else base="$base/download/$version"; fi
asset="seamgraph_${os}_${arch}.tar.gz"
printf '%s\n' "Downloading $repository ($version, $os/$arch)…"
fetch() { curl --proto '=https' --tlsv1.2 --fail --silent --show-error --location --retry 3 "$1" -o "$2"; }
fetch "$base/$asset" "$tmp/$asset"
fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS"
expected=$(awk -v name="$asset" '$2 == name {print $1}' "$tmp/SHA256SUMS")
[ "${#expected}" -eq 64 ] || fail 'missing or invalid release checksum'
case "$expected" in *[!a-fA-F0-9]*) fail 'invalid release checksum' ;; esac
if [ "$checksum" = sha256sum ]; then actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
else actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}'); fi
[ "$actual" = "$expected" ] || fail 'checksum mismatch; nothing installed'
# Extract only the expected binary, never arbitrary archive paths.
tar -xzf "$tmp/$asset" -C "$tmp" seamgraph
[ -f "$tmp/seamgraph" ] && [ ! -L "$tmp/seamgraph" ] || fail 'archive does not contain a regular binary'
mkdir -p "$bin_dir"
staged=$(mktemp "$bin_dir/.seamgraph-install.XXXXXX")
install -m 755 "$tmp/seamgraph" "$staged"
mv -f "$staged" "$bin_dir/seamgraph"
staged=
printf '%s\n' "Installed $bin_dir/seamgraph"
"$bin_dir/seamgraph" version
case ":${PATH:-}:" in *":$bin_dir:"*) ;; *) printf '%s\n' "Add $bin_dir to PATH in your shell or devcontainer configuration." ;; esac
if [ -n "$languages" ]; then
  if ! "$bin_dir/seamgraph" doctor --languages "$languages"; then
    printf '%s\n' 'SeamGraph is installed. Provision the missing runtimes in your devcontainer and rerun doctor.' >&2
    exit 2
  fi
else
  printf '%s\n' 'Check the languages you use: seamgraph doctor --languages python,typescript'
fi
