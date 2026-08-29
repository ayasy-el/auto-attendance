GET {{HOST}}/api/notifikasi/mahasiswa?filterNotif=PRESENSI

HTTP 200

[Captures]
KULIAH: jsonpath "$[0].dataTerkait" regex /^([0-9]+)-[0-9]+$/
JENIS_SCHEMA: jsonpath "$[0].dataTerkait" regex /^[0-9]+-([0-9]+)$/
ID_NOTIFIKASI: jsonpath "$[0].idNotifikasi"