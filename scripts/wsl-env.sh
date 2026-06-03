#!/usr/bin/env bash
# Load luwen's go + node toolchains in a NON-interactive shell
# (Ubuntu ~/.bashrc early-returns when non-interactive, so we wire these up directly.)
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
export NVM_DIR="$HOME/.nvm"
# shellcheck disable=SC1091
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh" >/dev/null 2>&1
