#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$ROOT_DIR/.env"
TMP_DIR="$ROOT_DIR/.tmp"
COOKIE_JAR="$TMP_DIR/cookies.txt"

command -v hurl >/dev/null 2>&1 || { echo "hurl tidak ditemukan" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq tidak ditemukan" >&2; exit 1; }
[[ -f "$ENV_FILE" ]] || { echo ".env tidak ditemukan" >&2; exit 1; }

set -a
source "$ENV_FILE"
set +a

: "${HOST:?HOST belum diset}"
: "${CAS_HOST:?CAS_HOST belum diset}"
: "${USERNAME:?USERNAME belum diset}"
: "${PASSWORD:?PASSWORD belum diset}"


mkdir -p "$TMP_DIR"
rm -f "$COOKIE_JAR" 

HURL_COMMON=(
  --variables-file "$ENV_FILE"
  --cookie "$COOKIE_JAR"
  --cookie-jar "$COOKIE_JAR"
)


echo "[+] CAS login + ETHOL authentication..."

RESPONSE=$(hurl "${HURL_COMMON[@]}" "$ROOT_DIR/01-auth.hurl")
MAHASISWA=$(echo "$RESPONSE" | jq -r '.nomor')

echo "$RESPONSE" | jq
echo "[+] Mahasiswa: $MAHASISWA"

RESPONSE=$(hurl "${HURL_COMMON[@]}" "$ROOT_DIR/02-notifikasi-presensi.hurl")
KULIAH=$(echo "$RESPONSE" | jq -r '.[0].dataTerkait | split("-")[0]')
JENIS_SCHEMA=$(echo "$RESPONSE" | jq -r '.[0].dataTerkait | split("-")[1]')

echo "$RESPONSE" | jq
echo "[+] KULIAH: $KULIAH"
echo "[+] JENIS_SCHEMA: $JENIS_SCHEMA"

RESPONSE=$(hurl "${HURL_COMMON[@]}" \
  --variable "KULIAH=$KULIAH" \
  --variable "JENIS_SCHEMA=$JENIS_SCHEMA" \
  "$ROOT_DIR/03-detail.hurl")
KULIAH_ASAL=$(echo "$RESPONSE" | jq -r '.[0].kuliah_asal')

echo "$RESPONSE" | jq
echo "[+] KULIAH_ASAL: $KULIAH_ASAL"
echo "[+] Presensi aktif..."

RESPONSE=$(hurl "${HURL_COMMON[@]}" \
  --variable "KULIAH=$KULIAH" \
  --variable "JENIS_SCHEMA=$JENIS_SCHEMA" \
 "$ROOT_DIR/04-presensi-aktif.hurl")
PRESENSI_KEY=$(echo "$RESPONSE" | jq -r '.[] | select(.open == 1) | .key')

echo "$RESPONSE" | jq
echo "[+] PRESENSI_KEY : $PRESENSI_KEY"
echo "[+] Submit presensi..."

hurl "${HURL_COMMON[@]}" \
  --variable "KULIAH=$KULIAH" \
  --variable "JENIS_SCHEMA=$JENIS_SCHEMA" \
  --variable "KULIAH_ASAL=$KULIAH_ASAL" \
  --variable "MAHASISWA=$MAHASISWA" \
  --variable "PRESENSI_KEY=$PRESENSI_KEY" \
  "$ROOT_DIR/05-presensi-submit.hurl"

echo "[+] Verify presensi..."

hurl "${HURL_COMMON[@]}" \
  --variable "KULIAH=$KULIAH" \
  --variable "JENIS_SCHEMA=$JENIS_SCHEMA" \
  --variable "MAHASISWA=$MAHASISWA" \
  "$ROOT_DIR/06-presensi-verify.hurl"

echo "[✓] ETHOL PRESENCE FLOW COMPLETED"
