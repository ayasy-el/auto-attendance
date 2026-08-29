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

GET {{HOST}}/api/auth/cas-callback?ticket={{CAS_TICKET}}

HTTP *


# ============================================================
# 3. VALIDATE ETHOL SESSION
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
# 4. FILTER NOTIFICATION
# ============================================================

GET {{HOST}}/api/notifikasi/mahasiswa?filterNotif=PRESENSI

HTTP 200

[Captures]
KULIAH: jsonpath "$[0].dataTerkait" regex /^([0-9]+)-[0-9]+$/
JENIS_SCHEMA: jsonpath "$[0].dataTerkait" regex /^[0-9]+-([0-9]+)$/
ID_NOTIFIKASI: jsonpath "$[0].idNotifikasi"

# ============================================================
# 5. GET COURSE DETAIL (ASAL_KULIAH)
# ============================================================

GET {{HOST}}/api/kuliah/by-kuliah-js?kuliah={{KULIAH}}&jenisSchema={{JENIS_SCHEMA}}
HTTP 200

[Captures]
KULIAH_ASAL: jsonpath "$[0].kuliah_asal"


# ============================================================
# 6. ACTIVE PRESENCE
# ============================================================

GET {{HOST}}/api/presensi/aktif-kuliah?kuliah={{KULIAH}}&jenis_schema={{JENIS_SCHEMA}}
HTTP 200

[Asserts]
jsonpath "$" exists

[Captures]
PRESENSI_KEY: jsonpath "$[?(@.open == 1)].key"


# ============================================================
# 7. SUBMIT PRESENCE
# ============================================================

POST {{HOST}}/api/presensi/mahasiswa
Content-Type: application/json

{
  "kuliah": {{KULIAH}},
  "jenis_schema": {{JENIS_SCHEMA}},
  "mahasiswa": {{MAHASISWA}},
  "key": "{{PRESENSI_KEY}}",
  "kuliah_asal": {{KULIAH_ASAL}}
}

HTTP 200

[Asserts]
jsonpath "$.sukses" == true


# ============================================================
# 8. VERIFY PRESENCE
# ============================================================

GET {{HOST}}/api/presensi/riwayat?kuliah={{KULIAH}}&jenis_schema={{JENIS_SCHEMA}}&nomor={{MAHASISWA}}

HTTP 200

[Asserts]
jsonpath "$" exists