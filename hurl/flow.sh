#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$ROOT_DIR/.env"
TMP_DIR="$ROOT_DIR/.tmp"
COOKIE_JAR="$TMP_DIR/cookies.txt"
USER_JSON="$TMP_DIR/user.json"
ACTIVE_JSON="$TMP_DIR/active-presensi.json"

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
: "${TAHUN:?TAHUN belum diset}"
: "${SEMESTER:?SEMESTER belum diset}"
: "${KULIAH:?KULIAH belum diset}"
: "${JENIS_SCHEMA:?JENIS_SCHEMA belum diset}"

mkdir -p "$TMP_DIR"
rm -f "$COOKIE_JAR" "$USER_JSON" "$ACTIVE_JSON"
trap 'rm -f "$COOKIE_JAR" "$USER_JSON" "$ACTIVE_JSON"' EXIT

HURL_COMMON=(
  --variables-file "$ENV_FILE"
  --cookie "$COOKIE_JAR"
  --cookie-jar "$COOKIE_JAR"
)

echo "[+] CAS login + ETHOL authentication..."
hurl "${HURL_COMMON[@]}" "$ROOT_DIR/01-auth.hurl" > "$USER_JSON"

MAHASISWA="$(jq -er '.nomor' "$USER_JSON")"
echo "[+] Mahasiswa: $MAHASISWA"

echo "[+] Daftar kuliah..."
hurl "${HURL_COMMON[@]}" "$ROOT_DIR/02-kuliah.hurl"

echo "[+] Jadwal..."
hurl "${HURL_COMMON[@]}" "$ROOT_DIR/03-jadwal.hurl"

echo "[+] Detail kuliah..."
hurl "${HURL_COMMON[@]}" "$ROOT_DIR/04-detail.hurl"

echo "[+] Riwayat presensi..."
hurl "${HURL_COMMON[@]}" \
  --variable "MAHASISWA=$MAHASISWA" \
  "$ROOT_DIR/05-presensi-history.hurl"

echo "[+] Presensi aktif..."
hurl "${HURL_COMMON[@]}" "$ROOT_DIR/06-presensi-aktif.hurl" > "$ACTIVE_JSON"

PRESENSI_KEY="$(jq -er '[.[] | select(.open == 1) | .key][0]' "$ACTIVE_JSON")"
echo "[+] PRESENSI_KEY berhasil diperoleh"

echo "[+] Submit presensi..."
hurl "${HURL_COMMON[@]}" \
  --variable "MAHASISWA=$MAHASISWA" \
  --variable "PRESENSI_KEY=$PRESENSI_KEY" \
  "$ROOT_DIR/07-presensi-submit.hurl"

echo "[+] Verify presensi..."
hurl "${HURL_COMMON[@]}" \
  --variable "MAHASISWA=$MAHASISWA" \
  "$ROOT_DIR/08-presensi-verify.hurl"

echo "[✓] ETHOL PRESENCE FLOW COMPLETED"
