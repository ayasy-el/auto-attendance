# ETHOL - PRESENCE HISTORY

GET {{HOST}}/api/presensi/riwayat?kuliah={{KULIAH}}&jenis_schema={{JENIS_SCHEMA}}&nomor={{MAHASISWA}}
HTTP 200

[Asserts]
jsonpath "$" exists

GET {{HOST}}/api/presensi/terakhir-kuliah?kuliah={{KULIAH}}&jenis_schema={{JENIS_SCHEMA}}
HTTP 200

[Asserts]
jsonpath "$" exists
