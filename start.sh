#!/usr/bin/env bash
# Dijalankan di dalam container Pterodactyl.
# Download binary agent sesuai arsitektur, lalu jalankan.
set -euo pipefail

REPO="malasahjagotwin/rtunnel"
VERSION="${RTUNNEL_VERSION:-latest}"

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "arsitektur tidak didukung: $ARCH" >&2; exit 1 ;;
esac

: "${TUNNEL_TOKEN:?set variabel TUNNEL_TOKEN di menu Startup panel}"

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

# agent membaca port dari env SERVER_PORT yang di-set Wings (fallback 14776).
exec ./agent.bin
