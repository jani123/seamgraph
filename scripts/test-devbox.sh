#!/bin/sh
# Exercise installation into a separate consumer, without the development devbox.
set -eu
repository=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
package=${1:-"git+file:$repository#seamgraph"}
consumer=$(mktemp -d)
trap 'rm -rf "$consumer"' EXIT
cp -R "$repository/testdata/projects/python/." "$consumer/"
cd "$consumer"
devbox init
devbox add "$package" 'python@3.12'
devbox run seamgraph version
devbox run seamgraph doctor --languages python
devbox run seamgraph scan --root . --format json > snapshot.json
devbox run python - <<'PY'
import json
from pathlib import Path

snapshot = json.loads(Path("snapshot.json").read_text())
assert len(snapshot["modules"]) == 2, snapshot
assert len(snapshot["seams"]) == 1, snapshot
PY
devbox run seamgraph check --root .
