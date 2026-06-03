#!/usr/bin/env bash
source "$(dirname "$0")/wsl-env.sh"
echo "go:   $(command -v go)"
go version
echo "node: $(command -v node)"
node --version
echo "npm:  $(command -v npm)"
npm --version
