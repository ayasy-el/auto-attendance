# ETHOL - SCHEDULE

POST {{HOST}}/api/kuliah/hari-kuliah-in
Content-Type: application/json

{
  "kuliahs": [
    {
      "nomor": {{KULIAH}},
      "jenisSchema": {{JENIS_SCHEMA}}
    }
  ],
  "tahun": {{TAHUN}},
  "semester": {{SEMESTER}}
}

HTTP 200

[Asserts]
jsonpath "$" exists
