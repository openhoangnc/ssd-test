#!/bin/sh
# ssd-test — run (default) or install.
#
#   curl -fsSL https://raw.githubusercontent.com/openhoangnc/ssd-test/main/run.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/openhoangnc/ssd-test/main/run.sh | sh -s -- --size 1G --output /tmp/r.html
#   curl -fsSL https://raw.githubusercontent.com/openhoangnc/ssd-test/main/run.sh | INSTALL=1 sh
#
# By default the binary is fetched into /tmp/ssd-test, exec'd, and reused on
# the next run (refreshed when older than 24h). Set INSTALL=1 to copy it into
# $DEST (default ~/.local/bin) and exit instead.
#
# Environment:
#   INSTALL=1    install permanently to $DEST instead of running
#   DEST         install directory (default: $HOME/.local/bin)
#   FORCE=1      redownload even if the cache is fresh
#   VERSION      pin a specific release tag (default: latest)
set -eu

REPO="openhoangnc/ssd-test"
VERSION="${VERSION:-latest}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 1 ;;
esac
case "$OS" in
  linux|darwin) ;;
  *) echo "unsupported OS: $OS (Windows: download from the Releases page)" >&2; exit 1 ;;
esac

BIN="ssd-test-${OS}-${ARCH}"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${BIN}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${BIN}"
fi

if [ "${INSTALL:-0}" = "1" ]; then
  CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/ssd-test"
else
  CACHE_DIR="/tmp/ssd-test"
fi
mkdir -p "$CACHE_DIR"
EXE="$CACHE_DIR/$BIN"

needs_download=0
if [ ! -x "$EXE" ]; then
  needs_download=1
elif [ "${FORCE:-0}" = "1" ]; then
  needs_download=1
elif find "$EXE" -mtime +1 2>/dev/null | grep -q .; then
  needs_download=1
fi

if [ "$needs_download" = "1" ]; then
  echo "ssd-test: fetching $BIN ($VERSION)" >&2
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$URL" -o "$EXE.tmp"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$URL" -O "$EXE.tmp"
  else
    echo "neither curl nor wget found on PATH" >&2
    exit 1
  fi
  chmod +x "$EXE.tmp"
  mv "$EXE.tmp" "$EXE"
fi

if [ "${INSTALL:-0}" = "1" ]; then
  DEST="${DEST:-$HOME/.local/bin}"
  mkdir -p "$DEST"
  cp "$EXE" "$DEST/ssd-test"
  chmod +x "$DEST/ssd-test"
  echo "Installed: $DEST/ssd-test"
  case ":$PATH:" in
    *":$DEST:"*) ;;
    *) echo "Note: $DEST is not on \$PATH. Add it to your shell config to run 'ssd-test' directly." ;;
  esac
  exit 0
fi

if [ ! -t 0 ] && [ -r /dev/tty ]; then
  exec "$EXE" "$@" </dev/tty
else
  exec "$EXE" "$@"
fi
