# ETHOL - COURSE DETAIL

GET {{HOST}}/api/kuliah/by-kuliah-js?kuliah={{KULIAH}}&jenisSchema={{JENIS_SCHEMA}}
HTTP 200

[Asserts]
jsonpath "$" exists
