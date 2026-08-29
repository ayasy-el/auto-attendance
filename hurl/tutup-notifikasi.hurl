PUT {{HOST}}/api/notifikasi/mahasiswa-baca-notif
Content-Type: application/json

{
  "idNotifikasi": {{ID_NOTIFIKASI}}
}

HTTP 201

[Asserts]
jsonpath "$.success" == true
