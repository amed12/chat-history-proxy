# Chat History Archive Service

Layanan Golang mandiri (standalone) ini berfungsi untuk mengarsipkan pesan ruang obrolan Qiscus dan menyajikannya ke aplikasi klien (misal: Flutter) saat *sessional mode* token telah kedaluwarsa.

## Fitur Utama
- Menerima Webhook Qiscus "Mark as Resolved" dengan pengamanan **HMAC-SHA256 signature**.
- Menarik riwayat lengkap chat dari Qiscus REST API.
- Menyimpan arsip obrolan dalam bentuk JSONB di **PostgreSQL**.
- Endpoint HTTP (dilindungi JWT) untuk permintaan riwayat obrolan dari klien.

---

## 🚀 Cara Menjalankan Aplikasi di Lokal (Menggunakan Docker)

Proyek ini telah dikonfigurasi dengan Docker dan Docker Compose. Metode ini adalah cara paling mudah untuk menjalankan aplikasi dan database PostgreSQL secara bersamaan di komputer lokal Anda tanpa harus menginstal PostgreSQL secara manual.

### Prasyarat
- Pastikan Anda sudah menginstal [Docker Desktop](https://www.docker.com/products/docker-desktop/) atau OrbStack di komputer Anda.

### Langkah-Langkah Menjalankan (Docker)

1. **Konfigurasi Variabel Lingkungan**
   Konfigurasi bawaan (`DATABASE_URL`, kredensial Qiscus tes, dll) sudah diatur di dalam file `docker-compose.yml`. 
   Jika Anda ingin mengetesnya dengan kredensial Qiscus asli Anda, Anda dapat langsung mengedit bagian `environment` milik servis `app` di dalam `docker-compose.yml`.

2. **Menjalankan Aplikasi dan Database**
   Buka terminal di direktori proyek ini, lalu jalankan perintah berikut untuk mem-*build* dan menjalankan container di *background*:
   ```bash
   docker-compose up -d --build
   ```
   > *Catatan: Skrip migrasi database (`internal/database/migrations.sql`) secara otomatis akan dieksekusi ketika container PostgreSQL pertama kali berjalan, sehingga tabel `chat_archives` akan langsung tersedia tanpa perlu migrasi manual.*

3. **Melihat Log Aplikasi**
   Untuk memastikan aplikasi telah berjalan dan berhasil terhubung ke database, cek log aplikasi dengan:
   ```bash
   docker-compose logs -f app
   ```
   Log tersebut akan menampilkan `database: connected` dan `server starting on :8080` jika proses *startup* berjalan mulus.

4. **Menguji Layanan (Health Check)**
   Untuk memverifikasi bahwa server backend telah aktif, Anda bisa memanggil endpoint *health check* menggunakan browser, Postman, atau Terminal:
   ```bash
   curl http://localhost:8080/health
   ```
   Respons yang benar adalah: `{"status":"ok"}`.

5. **Menghentikan Layanan**
   Untuk mematikan container aplikasi dan database:
   ```bash
   docker-compose down
   ```
   Jika Anda juga ingin menghapus seluruh data yang sudah tersimpan di database lokal Anda (reset *database volumes*), tambahkan flag `-v`:
   ```bash
   docker-compose down -v
   ```

---

## 🛠 Cara Menjalankan Aplikasi Secara Manual (Tanpa Docker)

Jika Anda ingin menjalankan atau men-debug (*debug*) aplikasi langsung menggunakan Go:

1. Pastikan PostgreSQL berjalan dan Anda telah menjalankan/mengeksekusi kueri yang ada di file `internal/database/migrations.sql`.
2. Buat file konfigurasi `.env` dengan menyalinnya dari *template*:
   ```bash
   cp .env.example .env
   ```
3. Sesuaikan nilai-nilai di dalam file `.env` (khususnya `DATABASE_URL` dan konfigurasi Qiscus).
4. Unduh *dependencies* dan jalankan servernya:
   ```bash
   go mod tidy
   go run ./cmd/server
   ```
