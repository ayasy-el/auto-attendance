# ETHOL - COURSE DETAIL

GET {{HOST}}/api/kuliah/by-kuliah-js?kuliah={{KULIAH}}&jenisSchema={{JENIS_SCHEMA}}
HTTP 200

[Captures]
KULIAH_ASAL: jsonpath "$[0].kuliah_asal"