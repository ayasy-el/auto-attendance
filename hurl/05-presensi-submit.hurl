# ETHOL - SUBMIT PRESENCE

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
