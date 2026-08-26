# ============================================================
# ETHOL - FULL END-TO-END PRESENCE FLOW
# ============================================================
#
# External variables (.env):
#
#   HOST
#   CAS_HOST
#   USERNAME
#   PASSWORD
#   TAHUN
#   SEMESTER
#   KULIAH
#   JENIS_SCHEMA
#
# Runtime variables:
#
#   CAS_LT
#   CAS_TICKET
#   MAHASISWA
#   PRESENSI_KEY
#
# Flow:
#
#   CAS login
#       ↓
#   ETHOL CAS callback
#       ↓
#   validasi-token
#       ↓
#   capture mahasiswa nomor
#       ↓
#   daftar kuliah
#       ↓
#   jadwal
#       ↓
#   detail kuliah
#       ↓
#   riwayat presensi
#       ↓
#   presensi terakhir
#       ↓
#   presensi aktif
#       ↓
#   capture active key
#       ↓
#   POST presensi mahasiswa
#       ↓
#   verify riwayat
#
# ============================================================


# ============================================================
# 0. CAS LOGIN PAGE
# ============================================================

GET {{CAS_HOST}}/cas/login?service={{HOST}}/api/auth/cas-callback
HTTP 200

[Captures]
CAS_LT: xpath "string(//*[@name='lt']/@value)"


# ============================================================
# 1. CAS LOGIN
# ============================================================

POST {{CAS_HOST}}/cas/login?service={{HOST}}/api/auth/cas-callback

[FormParams]
username: {{USERNAME}}
password: {{PASSWORD}}
lt: {{CAS_LT}}
_eventId: submit
submit: LOGIN

HTTP 302

[Captures]
CAS_TICKET: header "Location" regex "ticket=([^&]+)"


# ============================================================
# 2. ETHOL CAS CALLBACK
# ============================================================
#
# CAS POST memberikan:
#
# Location:
# https://ethol.pens.ac.id/api/auth/cas-callback?ticket=ST-...
#
# Ticket tersebut harus diteruskan ke ETHOL.
#
# ============================================================

GET {{HOST}}/api/auth/cas-callback?ticket={{CAS_TICKET}}

HTTP *


# ============================================================
# 3. VALIDATE ETHOL SESSION
# ============================================================
#
# Response user menjadi sumber MAHASISWA.
#
# Frontend menggunakan user.nomor sebagai:
#
#   mahasiswa: o.nomor
#
# Jadi tidak perlu MAHASISWA di .env.
#
# ============================================================

GET {{HOST}}/api/auth/validasi-token

HTTP 200

[Captures]
MAHASISWA: jsonpath "$.nomor"

[Asserts]
jsonpath "$.nomor" exists
jsonpath "$.nipnrp" exists
jsonpath "$.nama" exists
jsonpath "$.hakAkses" exists


# ============================================================
# 4. GET MAHASISWA'S COURSES
# ============================================================

GET {{HOST}}/api/kuliah?tahun={{TAHUN}}&semester={{SEMESTER}}

HTTP 200

[Asserts]
jsonpath "$" exists


# ============================================================
# 5. GET SCHEDULE
# ============================================================
#
# Endpoint:
#
# POST /api/kuliah/hari-kuliah-in
#
# ============================================================

POST {{HOST}}/api/kuliah/hari-kuliah-in
Content-Type: application/json

{
  "kuliahs": [
    {
      "nomor": {{KULIAH}},
      "jenisSchema": {{JENIS_SCHEMA}}
    }
  ],
  "tahun": {{TAHUN}},
  "semester": {{SEMESTER}}
}

HTTP 200

[Asserts]
jsonpath "$" exists


# ============================================================
# 6. COURSE DETAIL
# ============================================================

GET {{HOST}}/api/kuliah/by-kuliah-js?kuliah={{KULIAH}}&jenisSchema={{JENIS_SCHEMA}}

HTTP 200

[Asserts]
jsonpath "$" exists


# ============================================================
# 7. PRESENCE HISTORY
# ============================================================
#
# MAHASISWA berasal dari /auth/validasi-token.
#
# ============================================================

GET {{HOST}}/api/presensi/riwayat?kuliah={{KULIAH}}&jenis_schema={{JENIS_SCHEMA}}&nomor={{MAHASISWA}}

HTTP 200

[Asserts]
jsonpath "$" exists


# ============================================================
# 8. LAST PRESENCE
# ============================================================

GET {{HOST}}/api/presensi/terakhir-kuliah?kuliah={{KULIAH}}&jenis_schema={{JENIS_SCHEMA}}

HTTP 200

[Asserts]
jsonpath "$" exists


# ============================================================
# 9. ACTIVE PRESENCE
# ============================================================
#
# Frontend melakukan:
#
#   aktifKuliah(kuliah, jenis_schema)
#
# kemudian mencari:
#
#   data.find(x => x.open === 1)
#
# Objek tersebut yang menjadi active presence.
#
# key dari objek active tersebut dipakai saat submit.
#
# ============================================================

GET {{HOST}}/api/presensi/aktif-kuliah?kuliah={{KULIAH}}&jenis_schema={{JENIS_SCHEMA}}

HTTP 200

[Captures]
PRESENSI_KEY: jsonpath "$[?(@.open == 1)][0].key"

[Asserts]
jsonpath "$" exists


# ============================================================
# 10. SUBMIT PRESENCE
# ============================================================
#
# Equivalent frontend call:
#
# mahasiswaPresensi({
#   kuliah,
#   jenis_schema,
#   mahasiswa,
#   key,
#   kuliah_asal
# })
#
# MAHASISWA     -> captured from validasi-token
# PRESENSI_KEY  -> captured from aktif-kuliah
#
# ============================================================

POST {{HOST}}/api/presensi/mahasiswa
Content-Type: application/json

{
  "kuliah": {{KULIAH}},
  "jenis_schema": {{JENIS_SCHEMA}},
  "mahasiswa": {{MAHASISWA}},
  "key": "{{PRESENSI_KEY}}",
  "kuliah_asal": null
}

HTTP 200

[Asserts]
jsonpath "$.sukses" == true


# ============================================================
# 11. VERIFY PRESENCE
# ============================================================

GET {{HOST}}/api/presensi/riwayat?kuliah={{KULIAH}}&jenis_schema={{JENIS_SCHEMA}}&nomor={{MAHASISWA}}

HTTP 200

[Asserts]
jsonpath "$" exists