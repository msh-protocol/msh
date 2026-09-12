#!/usr/bin/env bash
set -e

# msh installer for macOS and Linux
# Usage: curl -fsSL https://raw.githubusercontent.com/msh-protocol/msh/main/scripts/install.sh | bash

REPO="msh-protocol/msh"
INSTALL_DIR="/usr/local/bin"

echo "==> Detecting system architecture..."
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64)
    ARCH="amd64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

case "$OS" in
  linux)
    OS="linux"
    ;;
  darwin)
    OS="darwin"
    ;;
  *)
    echo "Unsupported operating system: $OS"
    exit 1
    ;;
esac

echo "==> Finding latest release of msh..."
LATEST_TAG=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
  LATEST_TAG="v1.3.0"
fi

FILENAME="msh-${LATEST_TAG}-${OS}-${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${FILENAME}"

echo "==> Downloading msh ${LATEST_TAG} for ${OS}/${ARCH}..."
TMP_DIR="$(mktemp -d)"
trap 'rm -rf -- "$TMP_DIR"' EXIT

if ! curl -sSL -o "${TMP_DIR}/${FILENAME}" "$DOWNLOAD_URL"; then
  echo "Failed to download from $DOWNLOAD_URL"
  exit 1
fi

echo "==> Unpacking..."
tar -xzf "${TMP_DIR}/${FILENAME}" -C "${TMP_DIR}"

# Check install permissions
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP_DIR}/msh" "${INSTALL_DIR}/msh"
else
  echo "==> Requesting sudo to install to ${INSTALL_DIR}..."
  sudo mv "${TMP_DIR}/msh" "${INSTALL_DIR}/msh"
fi

chmod +x "${INSTALL_DIR}/msh"

echo "==> msh installed successfully!"
"${INSTALL_DIR}/msh" version
echo ""
echo "Try it now:"
echo "  msh git status"
echo "  msh npm test"
