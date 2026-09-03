# Auto Attendance

Aplikasi Go untuk login CAS sekali, mempertahankan cookie token, mengambil jadwal kuliah, dan menjalankan flow presensi dari `hurl/flow-all.hurl`.

## Menjalankan

```bash
cp config.yaml.example config.yaml
export USERNAME='...'
export PASSWORD='...'
go mod tidy
go run ./cmd/attendance -config config.yaml
```

## Menjalankan dengan Docker Compose

Pastikan `config.yaml` sudah berisi `host` dan `cas_host`, lalu buat `.env`:

```dotenv
USERNAME=
PASSWORD=
```

Jalankan aplikasinya:

```bash
docker compose up -d --build
docker compose logs -f auto-attendance
```

`config.yaml` di-mount read-only ke container dan `.env` hanya diberikan sebagai environment runtime; keduanya tidak disalin ke image.

`config.yaml` mendukung interval berikut:

- `schedule.outside_class_interval`: default `15m`
- `schedule.inside_class_interval`: default `2m`
- `schedule.active_start`: awal polling, default `08:00`
- `schedule.active_end`: akhir polling, default `18:00` (eksklusif)
- `schedule.refresh_schedule_interval`: default `24h`

Jadwal diambil dari `/api/kuliah` dan `/api/kuliah/hari-kuliah-in`. Saat tick, aplikasi membaca notifikasi `PRESENSI`, memuat detail kuliah, mencari presensi aktif, submit, lalu verifikasi riwayat. Presensi yang berhasil untuk kuliah yang sama ditandai selesai sampai hari berikutnya.

Jangan commit `config.yaml` jika berisi kredensial. Tambahkan `config.yaml` ke `.gitignore` bila konfigurasi lokal akan berisi password langsung.
