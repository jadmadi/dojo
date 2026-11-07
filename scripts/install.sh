#!/usr/bin/env sh
# Copyright 2026 The Dojo Authors
# Licensed under the Apache License, Version 2.0
#
# One-line installer for Dojo (Linux / macOS):
#   curl -fsSL https://raw.githubusercontent.com/jadmadi/dojo/main/scripts/install.sh | sh

set -eu

REPO="jadmadi/dojo"
BINARY="dojo"

# Determine target installation directory
if [ -n "${DOJO_INSTALL_DIR:-}" ]; then
  INSTALL_DIR="$DOJO_INSTALL_DIR"
elif [ -d "$HOME/.gemini/bin" ]; then
  INSTALL_DIR="$HOME/.gemini/bin"
else
  INSTALL_DIR="$HOME/.local/bin"
fi

mkdir -p "$INSTALL_DIR"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

case "$OS" in
  linux|darwin) ;;
  *)
    echo "Unsupported OS: $OS (use 'go install github.com/jadmadi/dojo/cmd/dojo@latest' on Windows)" >&2
    exit 1
    ;;
esac

echo "==> Installing $BINARY ($OS/$ARCH) to $INSTALL_DIR..."

# Try downloading prebuilt release binary first; fall back to go install if unavailable
ASSET_URL="https://github.com/${REPO}/releases/latest/download/${BINARY}_${OS}_${ARCH}.tar.gz"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

if command -v curl >/dev/null 2>&1 && curl -fsSL "$ASSET_URL" -o "$TMP_DIR/dojo.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP_DIR/dojo.tar.gz" -C "$TMP_DIR"
  install -m 0755 "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
  echo "==> Installed prebuilt $BINARY to $INSTALL_DIR/$BINARY"
elif command -v go >/dev/null 2>&1; then
  echo "==> Building $BINARY from source via Go toolchain..."
  GOBIN="$INSTALL_DIR" CGO_ENABLED=0 go install "github.com/${REPO}/cmd/dojo@latest"
  echo "==> Installed $BINARY to $INSTALL_DIR/$BINARY"
else
  echo "Error: Neither prebuilt archive at $ASSET_URL nor 'go' toolchain is available." >&2
  exit 1
fi

"$INSTALL_DIR/$BINARY" --version
