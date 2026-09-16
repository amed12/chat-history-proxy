# Chat History Proxy

Layanan Go mandiri (standalone), **tanpa database**, yang menjadi proxy read-only antara app
klien dan REST API admin Qiscus. Untuk app yang berjalan dalam mode Qiscus *sessional*: setelah
sebuah sesi selesai, aplikasi mengganti room chat aktifnya dan tidak lagi bisa membaca room lama
lewat token user biasa. Layanan ini menukar identitas (JWT user → kredensial server Qiscus) supaya
riwayat sesi lama tetap bisa dibaca — tanpa app perlu menyimpan salinan chat sendiri.

Ditulis generik: **ganti klien = ganti isi `.env`, bukan ganti kode.**

## Fitur Utama

- **Tanpa database, tanpa webhook.** Riwayat diambil langsung dari Qiscus saat diminta.
- `GET /api/v1/sessions` — daftar seluruh sesi (aktif + selesai) milik user yang terautentikasi.
- `GET /api/v1/sessions/{room_id}/messages` — transkrip penuh satu sesi, **hanya** untuk room
  milik user yang meminta (diverifikasi lewat daftar sesi, bukan dipercaya dari klien).
- Autentikasi JWT RS256 (public key dari environment) — token diterbitkan sistem auth klien
  sendiri, layanan ini hanya memvalidasinya.
- Cache pendek in-memory (default 60 detik) untuk daftar sesi per user, bisa dilewati per-request
  dengan `?fresh=1`.
- **Semua nilai per-deployment (App ID, secret, base URL, public key JWT, TTL cache, port) dibaca
  dari environment variable** — tidak ada yang hardcode di kode. Kredensial wajib membuat service
  gagal start dengan pesan jelas kalau kosong, bukan diam-diam jalan tanpa autentikasi.

## Menjalankan secara lokal

```bash
cp .env.example .env
# isi QISCUS_APP_ID, QISCUS_SECRET_KEY, JWT_PUBLIC_KEY (RSA PEM) — semua wajib,
# service akan gagal start dengan pesan jelas kalau salah satunya kosong

set -a; source .env; set +a
go run ./cmd/chat-history-proxy
```

Health check:

```bash
curl http://localhost:8081/health
```

### Testing tanpa auth backend klien asli

`cmd/devtools/gen-dev-jwt` menandatangani JWT dev pakai key lokal — buat
coba endpoint tanpa perlu backend auth klien beneran:

```bash
openssl genrsa -out .dev/jwt_private.pem 2048
openssl rsa -in .dev/jwt_private.pem -pubout -out .dev/jwt_public.pem
export JWT_PUBLIC_KEY="$(cat .dev/jwt_public.pem)"

TOKEN=$(go run ./cmd/devtools/gen-dev-jwt -key .dev/jwt_private.pem -sub some-user-id -ttl 168h)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8081/api/v1/sessions
```

`.dev/` sudah di-`.gitignore` — key ini murni lokal, jangan pernah dipakai di
deployment sungguhan.

## Menjalankan dengan Docker

```bash
cp docker-compose.yml.example docker-compose.yml
# isi environment: QISCUS_APP_ID, QISCUS_SECRET_KEY, JWT_PUBLIC_KEY

docker-compose up -d --build
docker-compose logs -f chat-history-proxy
```

`docker-compose.yml` sengaja di-`.gitignore` (lihat `.gitignore`) supaya kredensial asli tidak
pernah ter-commit — selalu mulai dari `docker-compose.yml.example`.

## Kontrak API

Semua endpoint di bawah `/api/v1/` butuh `Authorization: Bearer <JWT RS256>` dengan klaim `sub`
berisi user id klien (kunci yang sama dipakai app untuk `widget.setUser({ userId })`).

```
GET /api/v1/sessions
  -> { "data": [ { "room_id", "name", "started_at", "is_resolved", "last_message", "topic" } ], "next_cursor": null }

GET /api/v1/sessions/{room_id}/messages
  -> { "data": [ { "id", "sender_role", "sender_name", "type", "text", "payload", "created_at" } ], "next_cursor": null }
```

`room_id` yang bukan milik pemanggil token membalas `404` — bukan `403` — supaya tidak
membocorkan keberadaan room orang lain.

## Konfigurasi

| Env | Wajib | Default | Keterangan |
|---|---|---|---|
| `APP_PORT` | tidak | `8081` | Port HTTP |
| `QISCUS_APP_ID` | **ya** | — | App ID Qiscus milik klien |
| `QISCUS_SECRET_KEY` | **ya** | — | Secret server Qiscus, tidak pernah dikirim ke app klien |
| `QISCUS_BASE_URL` | tidak | `https://api3.qiscus.com` | |
| `JWT_PUBLIC_KEY` | **ya** | — | Public key RSA (PEM) untuk memverifikasi JWT dari auth klien |
| `CACHE_TTL_SECONDS` | tidak | `60` | Umur cache daftar sesi per user |

## Struktur kode

```
cmd/chat-history-proxy/main.go   # entry point, wiring
cmd/devtools/gen-dev-jwt/        # LOCAL DEV ONLY — generator JWT buat testing tanpa auth klien asli
internal/proxy/
  config/     # loader environment, gagal start kalau kredensial wajib kosong
  qiscus/     # klien REST admin Qiscus (get_user_rooms, load_comments)
  cache/      # TTL cache in-memory generik, tanpa dependency eksternal
  middleware/ # verifikasi JWT RS256
  handler/    # /api/v1/sessions, /api/v1/sessions/{room_id}/messages
internal/middleware/logger.go  # middleware generik
```

Detail arsitektur, alasan desain, dan batasan yang diketahui: lihat
[`docs/CHAT_HISTORY_PROXY.md`](./docs/CHAT_HISTORY_PROXY.md).

## Test

```bash
go build ./...
go vet ./...
go test ./...
```
