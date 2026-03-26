#!/bin/bash
set -e

REPO="fastclaw-ai/anyclaw"
INSTALL_DIR="/usr/local/bin"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version
if command -v curl &> /dev/null; then
  LATEST=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | cut -d'"' -f4)
elif command -v wget &> /dev/null; then
  LATEST=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | cut -d'"' -f4)
else
  echo "Error: curl or wget required"; exit 1
fi

if [ -z "$LATEST" ]; then
  echo "Error: failed to get latest version"; exit 1
fi

echo "Installing anyclaw ${LATEST} (${OS}/${ARCH})..."

FILENAME="anyclaw_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${FILENAME}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

if command -v curl &> /dev/null; then
  curl -sL "$URL" -o "${TMPDIR}/${FILENAME}"
else
  wget -q "$URL" -O "${TMPDIR}/${FILENAME}"
fi

tar xzf "${TMPDIR}/${FILENAME}" -C "$TMPDIR"

if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/anyclaw" "${INSTALL_DIR}/anyclaw"
else
  sudo mv "${TMPDIR}/anyclaw" "${INSTALL_DIR}/anyclaw"
fi

echo "Installed: $(anyclaw version)"
