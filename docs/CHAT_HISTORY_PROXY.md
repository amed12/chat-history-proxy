# Chat History Proxy

Layanan Go mandiri, **tanpa database**, yang berfungsi sebagai proxy read-only
antara aplikasi mobile/web klien dan REST API admin Qiscus. Layanan dirancang
generik dan tidak terikat pada klien tertentu, sehingga dapat digunakan kembali
untuk menampilkan riwayat percakapan lintas sesi pada aplikasi mode *sessional*.

## Kapan layanan ini relevan

Pada mode Qiscus *sessional*, aplikasi mengganti room chat aktif setiap kali
sesi selesai (*resolved*). Widget/SDK pada aplikasi hanya menyimpan referensi
ke **satu** room aktif. Setelah sesi lama selesai, aplikasi tidak lagi dapat
membaca isi room tersebut melalui token pengguna biasa karena token tersebut
tidak valid untuk room yang bukan room aktifnya.

Layanan ini menjembatani kebutuhan tersebut dengan menukar identitas (JWT milik
pengguna → kredensial **server** Qiscus) agar riwayat sesi lama milik pengguna
dapat dibaca tanpa aplikasi menyimpan salinan percakapan sendiri.

## Arsitektur

- **Tanpa database, tanpa webhook.** Riwayat diambil langsung dari Qiscus
  saat diminta melalui `GET /api/v1/sessions` dan `GET
  /api/v1/sessions/{room_id}/messages`), bukan disalin ke storage sendiri.
- Cache in-memory pendek (default 60 detik) per user, untuk membatasi beban ke
  Qiscus. Cache dapat dilewati per-request dengan `?fresh=1`, yang digunakan
  aplikasi setelah membuat room baru agar tidak menunggu TTL cache.
- Autentikasi: JWT RS256, public key dari environment. Token **diterbitkan
  oleh backend autentikasi klien sendiri** — layanan ini hanya memvalidasinya, tidak
  pernah menerbitkan token.
- Verifikasi kepemilikan room dilakukan sebelum mengirim transkrip — `sub`
  dari JWT harus memiliki room tersebut di daftar sesi. Jika tidak, layanan
  mengembalikan `404` (bukan `403`) agar keberadaan room pengguna lain tidak
  terungkap.

## Konfigurasi (semua lewat environment variable)

| Env | Wajib | Default | Keterangan |
|---|---|---|---|
| `APP_PORT` | tidak | `8081` | Port HTTP |
| `QISCUS_APP_ID` | **ya** | — | App ID Qiscus klien |
| `QISCUS_SECRET_KEY` | **ya** | — | Secret server Qiscus, tidak pernah dikirim ke klien app |
| `QISCUS_BASE_URL` | tidak | `https://api3.qiscus.com` | |
| `JWT_PUBLIC_KEY` | **ya** | — | Public key RSA (PEM) untuk memverifikasi JWT dari auth klien |
| `CACHE_TTL_SECONDS` | tidak | `60` | Umur cache daftar sesi per user |

Tidak ada nilai default untuk kredensial (`QISCUS_APP_ID`,
`QISCUS_SECRET_KEY`, `JWT_PUBLIC_KEY`). Apabila salah satu nilai kosong,
layanan **gagal dijalankan** dengan pesan yang menyebut environment variable
yang perlu dilengkapi. Konfigurasi untuk klien lain cukup dilakukan melalui
`.env`, tanpa perubahan kode.

Mulai dari template:

```bash
cp .env.example .env
# Tetapkan QISCUS_APP_ID, QISCUS_SECRET_KEY, QISCUS_BASE_URL (jika berbeda dari
# default), dan JWT_PUBLIC_KEY (public key milik backend autentikasi klien;
# key ini tidak dibuat oleh layanan).

set -a; source .env; set +a
go run ./cmd/chat-history-proxy
```

## Kontrak API

```
GET /api/v1/sessions
  Header: Authorization: Bearer <JWT RS256, klaim `sub` = user_id klien>
  -> { "data": [ { "room_id", "name", "started_at", "is_resolved", "last_message", "topic" } ], "next_cursor": null }

GET /api/v1/sessions/{room_id}/messages
  -> { "data": [ { "id", "sender_role", "sender_name", "type", "text", "payload", "created_at" } ], "next_cursor": null }
```

`topic` kosong apabila aplikasi pemanggil tidak menyimpannya secara eksplisit
melalui `qiscus.updateChatRoom(..., extras)`. Lihat repo widget/aplikasi untuk
pola dialog topik sebelum percakapan yang digunakan pada contoh implementasi.

## Batasan yang diketahui

- **`get_user_rooms` Qiscus tidak menyediakan `last_comment`/`room_created_at`.**
  `started_at` dan `last_message` diisi lewat **satu panggilan `load_comments`
  tambahan per room** (N+1 untuk N room) — biaya nyata, diterima untuk skala
  jumlah room per user yang wajar (bukan ribuan).
- **`load_comments` Qiscus mengembalikan pesan NEWEST FIRST (descending)**,
  bukan oldest-first. Setiap perubahan pada `internal/proxy/qiscus/client.go`
  perlu memverifikasi urutan tersebut melalui data representatif yang memiliki
  banyak pesan dan rentang waktu yang memadai. Fixture dua pesan sebelumnya
  tidak cukup untuk memvalidasi perilaku ini.
- Verifikasi terhadap Qiscus dilakukan menggunakan aplikasi Qiscus milik tim
  internal, bukan aplikasi produksi klien. Bentuk respons diperkirakan berlaku
  umum, namun perlu diverifikasi kembali apabila terdapat anomali pada
  implementasi klien baru.

## Struktur kode

```
cmd/chat-history-proxy/main.go   # entry point dan wiring
cmd/devtools/gen-dev-jwt/        # HANYA PENGEMBANGAN LOKAL — generator JWT untuk pengujian tanpa auth klien
internal/proxy/
  config/     # loader environment; gagal dijalankan jika kredensial wajib kosong
  qiscus/     # klien REST admin Qiscus (get_user_rooms, load_comments)
  cache/      # TTL cache in-memory generik tanpa dependency eksternal
  middleware/ # verifikasi JWT RS256
  handler/    # /api/v1/sessions, /api/v1/sessions/{room_id}/messages
internal/middleware/logger.go  # middleware generik yang dapat digunakan ulang dalam repo yang sama
```

## Test

```bash
go build ./...
go vet ./...
go test ./...
```
