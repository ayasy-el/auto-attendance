# ETHOL - COURSES

GET {{HOST}}/api/kuliah?tahun={{TAHUN}}&semester={{SEMESTER}}
HTTP 200

[Asserts]
jsonpath "$" exists
