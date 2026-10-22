#!/bin/sh
set -eu
: "${SEAMGRAPH_DEV_STATE:?Start with task dev so the session port is preserved}"
addr=${SEAMGRAPH_DEV_ADDR:-127.0.0.1:0}
if [ -s "$SEAMGRAPH_DEV_STATE" ]; then addr=$(cat "$SEAMGRAPH_DEV_STATE"); fi
# Dev Containers use Docker's localhost mapping through the fixed-target relay.
# Native development keeps the loopback-only, OS-assigned address by default.
if [ "${SEAMGRAPH_DEVCONTAINER:-}" = 1 ]; then
  addr=${SEAMGRAPH_DEV_ADDR:-0.0.0.0:8765}
  set -- --allow-network "$@"
fi
exec .build/air/seamgraph serve --root "${SEAMGRAPH_DEV_ROOT:-.}" \
  --addr "$addr" --listen-file "$SEAMGRAPH_DEV_STATE" "$@"
