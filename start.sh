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

# Wings set SERVER_IP/SERVER_PORT ke alokasi server ini, jadi agent ikut otomatis
# walau IP atau port beda. Baris ini ngeprint alamat yang harus dipakai client.
ADDR="${SERVER_IP:-<ip-publik>}:${SERVER_PORT:-14776}"
echo "listen di ${SERVER_IP:-0.0.0.0}:${SERVER_PORT:-14776}"
echo "di CT jalankan: rtclient -server $ADDR"

# agent membaca port dari env SERVER_PORT yang di-set Wings (fallback 14776).
exec ./agent.bin
