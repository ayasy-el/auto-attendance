# ETHOL - VERIFY PRESENCE

GET {{HOST}}/api/presensi/riwayat?kuliah={{KULIAH}}&jenis_schema={{JENIS_SCHEMA}}&nomor={{MAHASISWA}}
HTTP 200

[Asserts]
jsonpath "$" exists
