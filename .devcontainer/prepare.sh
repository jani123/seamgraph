#!/usr/bin/env bash
set -euo pipefail

# New named volumes belong to root. Only change the volume roots, never the
# shared checkout. The Dev Container CLI maps this user to the host uid.
sudo chown "$(id -u):$(id -g)" .devbox "$HOME/.cache" "$HOME/go"
# Preserve updater permissions if the CLI remaps the image user's uid/gid.
sudo chown -R "$(id -u):$(id -g)" "$NPM_CONFIG_PREFIX"
devbox run -- true
