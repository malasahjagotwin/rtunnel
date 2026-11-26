#!/usr/bin/env bash
# Dijalankan di dalam container Pterodactyl.
# Pakai agent.bin lokal kalau sudah ada; kalau tidak, unduh dari Releases.
set -euo pipefail

REPO="malasahjagotwin/rtunnel"
VERSION="${RTUNNEL_VERSION:-latest}"

if [ ! -x ./agent.bin ]; then
  ARCH="$(uname -m)"
  case "$ARCH" in
    x86_64|amd64) ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *) echo "arsitektur tidak didukung: $ARCH" >&2; exit 1 ;;
  esac

  if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/$REPO/releases/latest/download/agent-linux-$ARCH"
  else
    URL="https://github.com/$REPO/releases/download/$VERSION/agent-linux-$ARCH"
  fi

  echo "mengunduh agent dari $URL"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o agent.bin "$URL"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO agent.bin "$URL"
  else
    echo "butuh curl atau wget di dalam container" >&2; exit 1
  fi
  chmod +x agent.bin
fi

# agent membaca port dari env SERVER_PORT yang di-set Wings (fallback 14776).
exec ./agent.bin
