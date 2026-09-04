# Entra API

Backend Entra untuk pengelolaan event, pemesanan tiket, pembayaran, check-in, dan transaksi cashless. Digunakan oleh Entra Web dan Entra App.

## Teknologi

Go 1.26.4, Gin, PostgreSQL 16, Redis 7, Kafka, MinIO, dan sqlc.

## Layanan

| Layanan | Port bawaan | Tanggung jawab |
| --- | --- | --- |
| auth-service | 8081 | Akun, autentikasi JWT, profil, dan peran pengguna |
| event-service | 8082 | Event, venue, kategori, jenis tiket, dan kuota |
| ticket-service | 8083 | Pesanan, tiket, Midtrans, statistik, dan withdrawal |
| payment-service | 8084 | Payment intent dan simulasi pembayaran |
| cashless-service | 8085 | Dompet, top-up, pembayaran merchant, dan permintaan refund |
| gate-service | 8086 | Validasi tiket dan check-in |
| storage-service | 8087 | Unggah dan daftar media di MinIO |

Komunikasi antarlayanan menggunakan HTTP dan Kafka. Enam layanan memiliki database PostgreSQL terpisah; storage-service menggunakan MinIO.

## Prasyarat

- Go 1.26.4 atau lebih baru.
- Docker dengan Docker Compose.
- CLI golang-migrate dengan dukungan PostgreSQL.
- PowerShell untuk contoh perintah berikut.
- sqlc jika mengubah query SQL; kode hasil generasi sudah tersedia.

## Menjalankan secara lokal

### 1. Unduh dan siapkan konfigurasi

```powershell
git clone https://github.com/wibisanabama/entra-api.git
cd entra-api
Copy-Item .env.example .env
```

Lewati penyalinan jika `.env` sudah ada. Sesuaikan nilainya dan ganti `JWT_SECRET` serta `INTERNAL_SERVICE_SECRET`. Semua layanan harus menggunakan nilai secret yang sama.

Tidak semua layanan memuat `.env` secara otomatis. Agar konfigurasi berlaku konsisten, jalankan blok berikut pada setiap terminal sebelum menjalankan layanan:

```powershell
Get-Content .env | ForEach-Object {
    $line = $_.Trim()
    if ($line -and -not $line.StartsWith('#')) {
        $pair = $line -split '=', 2
        if ($pair.Count -eq 2) {
            [Environment]::SetEnvironmentVariable($pair[0].Trim(), $pair[1].Trim(), 'Process')
        }
    }
}
```

Gunakan format `KEY=value` tanpa tanda kutip atau komentar di akhir baris.

### 2. Jalankan infrastruktur dan migrasi

```powershell
docker compose up -d
docker compose ps
```

Compose hanya menjalankan PostgreSQL, Redis, Kafka, Zookeeper, dan MinIO, bukan layanan Go. Database dibuat saat volume PostgreSQL pertama kali diinisialisasi. Tunggu infrastruktur siap, lalu jalankan:

```powershell
foreach ($service in @('auth', 'event', 'ticket', 'payment', 'cashless', 'gate')) {
    $database = [Environment]::GetEnvironmentVariable("POSTGRES_DB_$($service.ToUpper())")
    $dsn = "postgres://$($env:POSTGRES_USER):$($env:POSTGRES_PASSWORD)@$($env:POSTGRES_HOST):$($env:POSTGRES_PORT)/$($database)?sslmode=$($env:POSTGRES_SSLMODE)"
    migrate -path "./$service-service/migrations" -database $dsn up
    if ($LASTEXITCODE -ne 0) { throw "Migrasi $service gagal" }
}
```

Nilai koneksi PostgreSQL harus sesuai dengan `docker-compose.yml`. Jika kredensial mengandung karakter khusus URI, gunakan DSN dengan kredensial yang sudah di-URL-encode.

### 3. Jalankan layanan

Dari root repositori, jalankan setiap perintah di terminal terpisah setelah memuat environment:

```powershell
go run ./auth-service/cmd/api
go run ./event-service/cmd/api
go run ./ticket-service/cmd/api
go run ./payment-service/cmd/api
go run ./cashless-service/cmd/api
go run ./gate-service/cmd/api
go run ./storage-service/cmd/api
```

Endpoint menggunakan awalan `/api/v1`. Definisi rute tersedia di `internal/handler/routes.go` pada masing-masing layanan.

## Integrasi

- Pembayaran tiket memakai Midtrans Sandbox. Tambahkan `MIDTRANS_SERVER_KEY` ke environment untuk checkout.
- payment-service menyediakan simulasi; URL payment intent yang dibuatnya bukan halaman pembayaran aktif.
- Pengiriman email memerlukan `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, dan `SMTP_PASS`.
- `STORAGE_PUBLIC_URL` harus dapat diakses browser atau perangkat yang menampilkan media.
- Permintaan internal menggunakan header `X-Internal-Secret`; permintaan pengguna menggunakan JWT Bearer.

## Build dan pengujian

```powershell
go build ./auth-service/... ./event-service/... ./ticket-service/... ./payment-service/... ./cashless-service/... ./gate-service/... ./storage-service/... ./shared/...
go test ./auth-service/... ./event-service/... ./ticket-service/... ./payment-service/... ./cashless-service/... ./gate-service/... ./storage-service/... ./shared/...
```

Untuk memperbarui kode query, jalankan `sqlc generate` dari direktori layanan yang memiliki `sqlc.yaml`. Jangan mengedit hasil generasi secara langsung.

## Struktur

- `<nama>-service/cmd/api/`: entry point layanan.
- `<nama>-service/internal/handler/`: rute dan handler HTTP.
- `<nama>-service/internal/service/`: logika bisnis.
- `<nama>-service/internal/repository/`: query SQL dan kode hasil sqlc.
- `<nama>-service/migrations/`: migrasi database.
- `shared/`: konfigurasi, middleware, database, Kafka, dan respons API.
- `scripts/`: inisialisasi database.

Konfigurasi Compose dan integrasi pembayaran saat ini ditujukan untuk pengembangan lokal, bukan konfigurasi produksi.
