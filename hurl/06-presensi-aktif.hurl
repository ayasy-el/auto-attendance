# ETHOL - ACTIVE PRESENCE

GET {{HOST}}/api/presensi/aktif-kuliah?kuliah={{KULIAH}}&jenis_schema={{JENIS_SCHEMA}}
HTTP 200

[Asserts]
jsonpath "$" exists
