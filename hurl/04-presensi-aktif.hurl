# ETHOL - ACTIVE PRESENCE

GET {{HOST}}/api/presensi/aktif-kuliah?kuliah={{KULIAH}}&jenis_schema={{JENIS_SCHEMA}}
HTTP 200

[Asserts]
jsonpath "$" exists

[Captures]
PRESENSI_KEY: jsonpath "$[?(@.open == 1)].key"
