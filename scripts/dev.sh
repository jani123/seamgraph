#!/bin/sh
set -eu
mkdir -p .build
# One state file per Air session keeps the OS-selected port across rebuilds.
SEAMGRAPH_DEV_STATE=$(mktemp "$PWD/.build/dev-listen.XXXXXX")
export SEAMGRAPH_DEV_STATE
air_pid=
cleanup() {
  if [ -n "$air_pid" ] && kill -0 "$air_pid" 2>/dev/null; then
    kill -INT "$air_pid" 2>/dev/null || true
    wait "$air_pid" 2>/dev/null || true
  fi
  rm -f "$SEAMGRAPH_DEV_STATE"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
.build/bin/air -c .air.toml -- "$@" &
air_pid=$!
wait "$air_pid"
