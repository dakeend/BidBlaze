#!/usr/bin/env bash
# Throwaway probe: locate go/node toolchains under the luwen WSL account.
echo "USER=$(whoami)"
echo "HOME=$HOME"
echo "SHELL=$SHELL"
echo "PATH=$PATH"
echo "--- go locations ---"
ls -d /usr/local/go/bin 2>/dev/null && /usr/local/go/bin/go version 2>/dev/null
ls "$HOME/go/bin" 2>/dev/null
command -v go && go version
echo "--- node / nvm ---"
ls -d "$HOME/.nvm" 2>/dev/null
ls "$HOME/.nvm/versions/node" 2>/dev/null
command -v node && node --version
command -v npm && npm --version
echo "--- rc files mentioning toolchains ---"
grep -nE 'nvm|go/bin|golang|/usr/local/go' "$HOME/.bashrc" "$HOME/.profile" "$HOME/.bash_profile" 2>/dev/null
