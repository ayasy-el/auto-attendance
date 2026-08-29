#!/usr/bin/env bun

// Standalone mock based on the HTTP behavior observed in dump.mitm.
// Run with: bun hurl/mock-server.js

const port = Number(Bun.env.PORT ?? 3000);
const student = {
  nomor: 31842,
  nipnrp: "2412300147",
  nama: "Raka Pratama",
  hakAkses: ["mahasiswa"],
};
const activeKey = "presensi-key-7f3a91";
const courses = [
  {
    nomor: 318574,
    kuliah_asal: 318574,
    jenisSchema: 4,
    matakuliah: { nomor: 307821, nama: "Praktikum Machine Learning", jenisSchemaMk: 4 },
    dosen: "Fajar Ramadhan",
    gelar_dpn: null,
    gelar_blk: "M.Kom.",
    nip_dosen: "198807162019031006",
    nomor_dosen: 3147,
    kode_kelas: "7 STr TKJ",
    pararel: "A",
  },
  {
    nomor: 318579,
    kuliah_asal: 318579,
    jenisSchema: 4,
    matakuliah: {
      nomor: 307826,
      nama: "Praktikum Algoritma dan Struktur Data",
      jenisSchemaMk: 4,
    },
    dosen: "Fajar Ramadhan",
    gelar_dpn: null,
    gelar_blk: "M.Kom.",
    nip_dosen: "198807162019031006",
    nomor_dosen: 3147,
    kode_kelas: "7 STr TKJ",
    pararel: "B",
  },
];
const attendance = new Map();

const json = (data, status = 200, headers = {}) => Response.json(data, { status, headers });
const html = (data, status = 200, headers = {}) =>
  new Response(data, {
    status,
    headers: { "content-type": "text/html; charset=utf-8", ...headers },
  });
const course = (id) => courses.find((item) => item.nomor === Number(id));

function casPage() {
  return `<!doctype html><html><head><title>CAS – Central Authentication Service</title></head><body><form method="post"><input name="lt" value="mock-lt"><input name="_eventId" value="submit"><input name="submit" value="LOGIN"></form></body></html>`;
}

async function handler(req) {
  const url = new URL(req.url);
  const path = url.pathname;

  if (path === "/cas/login" && req.method === "GET")
    return html(casPage(), 200, { "set-cookie": "JSESSIONID=mock-session; Path=/cas/" });
  if (path === "/cas/login" && req.method === "POST")
    return new Response(null, {
      status: 302,
      headers: { location: "/api/auth/cas-callback?ticket=ST-MOCK-r8K2" },
    });
  if (path === "/api/auth/cas-callback")
    return new Response("Found. Redirecting to /auth/cas", {
      status: 302,
      headers: { location: "/auth/cas", "set-cookie": "token=mock-token-82kd71; Path=/; HttpOnly" },
    });
  if (path === "/api/auth/validasi-token")
    return json({
      ...student,
      iat: Math.floor(Date.now() / 1000),
      exp: Math.floor(Date.now() / 1000) + 900,
    });

  if (path === "/api/kuliah" && req.method === "GET") return json(courses);
  if (path === "/api/kuliah/by-kuliah-js" && req.method === "GET")
    return json(
      course(url.searchParams.get("kuliah")) ? [course(url.searchParams.get("kuliah"))] : [],
    );
  if (path === "/api/kuliah/hari-kuliah-in" && req.method === "POST") {
    const body = await req.json();
    const id = body?.kuliahs?.[0]?.nomor;
    return json(
      Number(id) === 318574
        ? [
            {
              kuliah: 318574,
              hari: "Senin",
              jam_awal: "10:15",
              jam_akhir: "12:45",
              nomor_hari: 1,
              nomor_ruang: 417,
              ruang: "LAB-JAR-02",
            },
          ]
        : [
            {
              kuliah: 318579,
              hari: "Rabu",
              jam_awal: "13:30",
              jam_akhir: "16:00",
              nomor_hari: 3,
              nomor_ruang: null,
              ruang: null,
            },
          ],
    );
  }

  if (path === "/api/presensi/terakhir-kuliah") {
    const id = Number(url.searchParams.get("kuliah"));
    return json(
      id === 318579
        ? {
            ditemukan: true,
            tanggal: "2026-08-26 16:00:14",
            kuliah: id,
            jenisSchema: 4,
            key: activeKey,
            open: true,
            tanggal_format: "Rabu, 26 Agustus 2026 - 16:00:14",
          }
        : { ditemukan: false },
    );
  }
  if (path === "/api/presensi/aktif-kuliah")
    return json(
      Number(url.searchParams.get("kuliah")) === 318579
        ? [{ kuliah: 318579, key: activeKey, jenisSchema: 4, open: 1 }]
        : [],
    );
  if (path === "/api/presensi/mahasiswa" && req.method === "POST") {
    attendance.set(student.nomor, true);
    return json({ sukses: true, pesan: "Presensi berhasil disimpan" });
  }
  if (path === "/api/presensi/riwayat")
    return json(
      attendance.has(student.nomor)
        ? [
            {
              nomor: 4738261,
              tanggal: "26-08-2026 16:01:27",
              waktu_indonesia: "Rabu, 26 Agustus 2026 - 16:01:27",
              key: activeKey,
            },
          ]
        : [],
    );
  if (path === "/api/notifikasi/mahasiswa" && req.method === "GET")
    return json([{ idNotifikasi: 17, dataTerkait: "318579-4", tipe: "PRESENSI" }]);
  if (path === "/api/notifikasi/mahasiswa-baca-notif" && req.method === "PUT")
    return json({ success: true }, 201);

  return json({ error: "mock route not found", method: req.method, path }, 404);
}

console.log(`Mock API listening on http://127.0.0.1:${port}`);
Bun.serve({ port, fetch: handler });
