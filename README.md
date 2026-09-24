# Cinema Online Ticketing – Backend Development Test

**Mitra Kasih Perkasa (MKP) – 2025**

| | |
|---|---|
| **Nama Lengkap** | Muhammad Nur Firdaus Setiadi |
| **Repository** | https://github.com/mnfirdauss/mkp-cinema-ticketing |

---

## Isi Repository

| Test | Jawaban |
|---|---|
| **A. System Design** | [`docs/SYSTEM_DESIGN.md`](docs/SYSTEM_DESIGN.md)<br/>Diagram JPG: [topologi](docs/diagrams/01-system-topology.jpg), [flowchart pembelian](docs/diagrams/02-flowchart-pembelian.jpg), [flow refund/pembatalan & restok](docs/diagrams/03-flow-refund-pembatalan.jpg) |
| **B. Database Design** | [`docs/DATABASE_DESIGN.md`](docs/DATABASE_DESIGN.md), [ERD JPG](docs/diagrams/04-erd.jpg), script PostgreSQL [`db/schema.sql`](db/schema.sql) + [`db/seed.sql`](db/seed.sql) |
| **C. Skill Test** | API Go (Login + CRUD Jadwal Tayang dengan JWT Authorization) di folder `cmd/` & `internal/` |
| **Postman** | [`postman/MKP-Cinema-Ticketing.postman_collection.json`](postman/MKP-Cinema-Ticketing.postman_collection.json) |

![Topology](docs/diagrams/01-system-topology.jpg)

---

## Menjalankan API

### Opsi 1: Docker Compose (paling mudah)

```bash
docker compose up -d --build
# API      : http://localhost:8080
# Postgres : localhost:5432  (postgres/postgres, db: cinema). Schema & seed otomatis di-import.
```

### Opsi 2: Manual

Kebutuhan: Go 1.26+ dan PostgreSQL 14+.

```bash
createdb cinema
psql -d cinema -f db/schema.sql
psql -d cinema -f db/seed.sql

cp .env.example .env   # sesuaikan jika perlu
export $(cat .env | xargs)
go run ./cmd/api
```

### Test

```bash
go test ./...                                                      # unit test
npx newman run postman/MKP-Cinema-Ticketing.postman_collection.json # API test (server harus jalan)
```

## Akun Seed

Semua akun memakai password **`password123`**.

| Email | Role | Hak akses jadwal tayang |
|---|---|---|
| `admin@mkp.id` | super_admin | CRUD semua bioskop |
| `smg@mkp.id` | cinema_admin | CRUD hanya untuk bioskop Semarang (studio 1 & 2) |
| `budi@mail.com` | customer | Hanya melihat (list & detail) |

## Endpoint

Base URL: `http://localhost:8080/api/v1`. Semua endpoint kecuali login memerlukan header `Authorization: Bearer <token>`.

| Method | Path | Akses | Keterangan |
|---|---|---|---|
| POST | `/auth/login` | publik | Login dan mendapatkan JWT |
| GET | `/auth/me` | login | Profil user yang sedang login |
| GET | `/showtimes` | login | List jadwal. Filter: `movie_id`, `cinema_id`, `city_id`, `date` (YYYY-MM-DD), `status`, `page`, `limit` |
| GET | `/showtimes/{id}` | login | Detail jadwal beserta jumlah kursi tersedia/terjual |
| POST | `/showtimes` | admin | Buat jadwal |
| PUT | `/showtimes/{id}` | admin | Ubah jadwal |
| DELETE | `/showtimes/{id}` | admin | Hapus jadwal (soft delete) |

Contoh:

```bash
TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login \
  -d '{"email":"admin@mkp.id","password":"password123"}' | jq -r .data.access_token)

curl -X POST localhost:8080/api/v1/showtimes -H "Authorization: Bearer $TOKEN" \
  -d '{"movie_id":1,"studio_id":1,"start_time":"2026-12-01T19:00:00+07:00","price":50000,"status":"open"}'
```

Format response:

```json
{ "success": true, "message": "showtime created", "data": { ... }, "meta": { "page": 1, "limit": 20, "total": 1, "total_pages": 1 } }
```

### Aturan bisnis yang diterapkan

- **Otorisasi**: JWT HS256 dan RBAC. `customer` hanya bisa membaca. `cinema_admin` hanya bisa mengelola jadwal di studio milik bioskopnya sendiri (403 untuk bioskop lain).
- `end_time` dihitung otomatis dari `start_time` + durasi film + 15 menit waktu bersih-bersih studio.
- **Jadwal bentrok** di studio yang sama ditolak oleh exclusion constraint PostgreSQL (409).
- Saat jadwal dibuat, **inventory kursi** (`showtime_seats`) ikut dibuat otomatis dalam transaksi yang sama.
- Jika jadwal sudah memiliki kursi `held`/`sold`, film/studio/jam tayang tidak dapat diubah dan jadwal tidak dapat dihapus (409). Admin harus memakai alur pembatalan & refund, lihat [System Design §2.4](docs/SYSTEM_DESIGN.md#24-refund--pembatalan-dari-pihak-bioskop).
- Status `cancelled` tidak bisa di-set lewat CRUD karena pembatalan harus melalui alur refund.
- Login mengembalikan pesan error yang sama untuk email tidak terdaftar dan password salah, sehingga tidak bisa dipakai untuk mengecek email mana yang terdaftar (user enumeration).

## Struktur Project

```
cmd/api/              entrypoint HTTP server (graceful shutdown)
internal/
  auth/               JWT & bcrypt
  config/             konfigurasi dari environment variable
  database/           koneksi pgxpool
  handler/            HTTP handler + validasi input
  httpx/              helper response JSON
  middleware/         Authenticate, RequireRole, Logger, Recover
  model/              struct domain
  repository/         query PostgreSQL (pgx)
db/                   schema.sql, seed.sql
docs/                 dokumen system design & database design, diagram JPG
  diagrams/src/       sumber diagram (Mermaid/HTML), render ulang: python3 docs/diagrams/render.py
postman/              Postman collection
```

**Tech stack**: Go 1.26 (`net/http` dengan routing bawaan), pgx v5, golang-jwt v5, bcrypt, PostgreSQL 16, Docker.
