POST {{HOST}}/api/kontrak/setuju
Content-Type: application/json

{
  "kuliah": {{KULIAH}},
  "jenis_schema": {{JENIS_SCHEMA}}
}

[Asserts]
jsonpath "$.sukses" == true