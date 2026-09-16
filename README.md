# Entra API

Repositori backend berbasis arsitektur microservices untuk platform manajemen event, pemesanan tiket, sistem pembayaran, validasi gate check-in, dan transaksi cashless Entra.

## Arsitektur Layanan

Sistem terbagi ke dalam tujuh layanan independen yang berkomunikasi melalui REST API dan Apache Kafka:

| Layanan | Port | Deskripsi Fungsional |
| --- | --- | --- |
| `auth-service` | 8081 | Autentikasi JWT, manajemen akun, profil pengguna, dan kontrol akses berbasis peran (RBAC) |
| `event-service` | 8082 | Pengelolaan data event, kategori, kuota, dan tier tiket |
| `ticket-service` | 8083 | Pemesanan tiket, integrasi payment gateway Midtrans, dan penarikan dana organizer |
| `payment-service` | 8084 | Manajemen payment intent dan pemrosesan status pembayaran |
| `cashless-service` | 8085 | Pengelolaan dompet digital, top-up saldo, transaksi point-of-sale, dan pengajuan refund |
| `gate-service` | 8086 | Pemindaian tiket QR, validasi akses pintu masuk, dan pencegahan tiket ganda |
| `storage-service` | 8087 | Manajemen unggah dan distribusi media melalui object storage MinIO |

## Prasyarat Sistem

- Go versi 1.22 atau lebih baru
- Docker dan Docker Compose
- CLI golang-migrate (untuk migrasi database lokal)
- Git

## Konfigurasi Lingkungan

Salin berkas contoh konfigurasi lingkungan pada direktori utama:

```powershell
Copy-Item .env.example .env
```

Sesuaikan parameter berikut pada berkas `.env`:

| Parameter | Keterangan | Nilai Bawaan |
| --- | --- | --- |
| `JWT_SECRET` | Kunci enkripsi token autentikasi pengguna | Wajib diisi |
| `INTERNAL_SERVICE_SECRET` | Kunci autentikasi komunikasi antarlayanan | Wajib diisi |
| `MIDTRANS_SERVER_KEY` | Kunci server payment gateway Midtrans | Sesuai akun Midtrans |
| `MIDTRANS_CLIENT_KEY` | Kunci client payment gateway Midtrans | Sesuai akun Midtrans |
| `SMTP_HOST` | Host server SMTP untuk email reset kata sandi | sandbox.smtp.mailtrap.io |
| `SMTP_PORT` | Port server SMTP | 2525 |
| `SMTP_USER` | Nama pengguna autentikasi SMTP | Sesuai akun SMTP |
| `SMTP_PASS` | Kata sandi autentikasi SMTP | Sesuai akun SMTP |
| `SMTP_FROM` | Alamat pengirim email sistem | noreply@entra.id |

## Menjalankan Sistem

### 1. Menjalankan Infrastruktur

Jalankan container PostgreSQL, Redis, Kafka, Zookeeper, dan MinIO menggunakan Docker Compose:

```powershell
docker compose up -d
```

Pastikan seluruh container dalam status berjalan:

```powershell
docker compose ps
```

### 2. Menjalankan Migrasi Database

Jalankan migrasi skema database untuk setiap layanan yang memiliki database relasional:

```powershell
$services = @('auth', 'event', 'ticket', 'payment', 'cashless', 'gate')
foreach ($svc in $services) {
    $dbName = "entra_$svc"
    $dsn = "postgres://entra:entra_secret@localhost:5432/$($dbName)?sslmode=disable"
    migrate -path "./$svc-service/migrations" -database $dsn up
}
```

### 3. Menjalankan Layanan

Setiap layanan dijalankan dari direktori root repositori pada terminal terpisah:

```powershell
# Jalankan auth-service
go run ./auth-service/cmd/api

# Jalankan event-service
go run ./event-service/cmd/api

# Jalankan ticket-service
go run ./ticket-service/cmd/api

# Jalankan payment-service
go run ./payment-service/cmd/api

# Jalankan cashless-service
go run ./cashless-service/cmd/api

# Jalankan gate-service
go run ./gate-service/cmd/api

# Jalankan storage-service
go run ./storage-service/cmd/api
```

Seluruh endpoint layanan menggunakan awalan jalur `/api/v1`. Health check tersedia pada jalur `/health` untuk masing-masing port layanan.

## Kompilasi dan Pengujian

### Kompilasi Semua Layanan

```powershell
go build ./auth-service/... ./event-service/... ./ticket-service/... ./payment-service/... ./cashless-service/... ./gate-service/... ./storage-service/... ./shared/...
```

### Menjalankan Unit Test

```powershell
go test ./auth-service/... ./event-service/... ./ticket-service/... ./payment-service/... ./cashless-service/... ./gate-service/... ./storage-service/... ./shared/...
```

### Regenerasi Kode SQL (sqlc)

Jika terdapat perubahan query SQL, regenerasi kode dilakukan melalui direktori masing-masing layanan yang memiliki konfigurasi `sqlc.yaml`:

```powershell
cd <nama-service>
sqlc generate
```

## Struktur Direktori

```text
entra-api/
├── auth-service/        # Layanan autentikasi dan otorisasi pengguna
├── event-service/       # Layanan katalog dan manajemen event
├── ticket-service/      # Layanan transaksi tiket dan penarikan saldo
├── payment-service/     # Layanan payment intent dan siklus pembayaran
├── cashless-service/    # Layanan dompet digital dan transaksi merchant
├── gate-service/        # Layanan validasi pintu masuk dan check-in QR
├── storage-service/     # Layanan integrasi media MinIO
├── shared/              # Pustaka bersama (konfigurasi, middleware, Kafka, model respons)
├── scripts/             # Skrip utilitas basis data
├── docker-compose.yml   # Definisi kontainer infrastruktur lokal
├── go.work              # Konfigurasi Go multi-module workspace
└── Makefile             # Otomasi tugas pengembangan
```
